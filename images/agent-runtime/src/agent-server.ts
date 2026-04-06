/**
 * Agent Runtime — Daemon Mode (agent-server.ts)
 *
 * HTTP server on :4096 wrapping Pi SDK via createAgentSession().
 * Endpoints: /prompt, /prompt/stream, /steer, /followup, /abort, /healthz, /status
 *
 * Loads operator extension + user extensions. Registers all providers
 * into ModelRegistry. Configures fallback models.
 */

import { readFileSync } from "node:fs";
import express from "express";
import { createAgentSession } from "@mariozechner/pi-coding-agent";
import {
  SessionManager,
  AuthStorage,
  ModelRegistry,
  SettingsManager,
} from "@mariozechner/pi-agent-core";
import type { OperatorConfig, AgentStatus, PromptRequest, SteerRequest, FollowupRequest } from "./types.js";
import { loadOperatorExtension } from "./operator-extension.js";

const PORT = 4096;
const CONFIG_PATH = "/etc/operator/config.json";

async function main() {
  // Load operator config
  const config: OperatorConfig = JSON.parse(
    readFileSync(CONFIG_PATH, "utf-8")
  );

  console.log(`[agent-server] Starting daemon agent`);
  console.log(`[agent-server] Primary model: ${config.primaryModel}`);
  console.log(`[agent-server] Providers: ${config.providers.map((p) => p.name).join(", ")}`);
  console.log(`[agent-server] Tools: ${config.tools.map((t) => t.name).join(", ")}`);

  // Setup Pi SDK components
  const authStorage = AuthStorage.create();
  const modelRegistry = ModelRegistry.create(authStorage);
  const sessionManager = SessionManager.create("/data/sessions");

  // Configure compaction
  const settingsManager = SettingsManager.inMemory({
    compaction: {
      enabled: config.compaction?.enabled ?? true,
      strategy: config.compaction?.strategy ?? "auto",
    },
  });

  // Register provider API keys
  for (const provider of config.providers) {
    const envKey = `${provider.name.toUpperCase()}_API_KEY`;
    const apiKey = process.env[envKey];
    if (apiKey) {
      authStorage.set(provider.name, apiKey);
      console.log(`[agent-server] Registered provider: ${provider.name}`);
    } else {
      console.warn(`[agent-server] WARNING: No API key found for provider ${provider.name} (env: ${envKey})`);
    }
  }

  // Create session
  const session = await createAgentSession({
    sessionManager,
    authStorage,
    modelRegistry,
    settingsManager,
  });

  // Load operator extension (tools, MCP, hooks, telemetry)
  await loadOperatorExtension(session, config);

  // Runtime state
  let busy = false;
  let lastOutput = "";
  let totalToolCalls = 0;
  let totalTokensUsed = 0;
  let totalCost = 0;
  let activeModel = config.primaryModel;

  // Fallback prompt handler
  async function handlePrompt(prompt: string): Promise<{
    output: string;
    toolCalls: number;
    tokensUsed: number;
    cost: number;
    model: string;
  }> {
    const models = [config.primaryModel, ...(config.fallbackModels ?? [])];

    for (const model of models) {
      try {
        session.setModel(model);
        activeModel = model;
        const result = await session.prompt(prompt);

        const output = typeof result === "string" ? result : JSON.stringify(result);
        return {
          output,
          toolCalls: 0, // Will be tracked by extension hooks
          tokensUsed: 0,
          cost: 0,
          model,
        };
      } catch (err: any) {
        const isRetryable = err?.status === 429 || err?.status >= 500;
        if (isRetryable && model !== models[models.length - 1]) {
          console.warn(`[agent-server] Model ${model} failed, trying next fallback...`);
          continue;
        }
        throw err;
      }
    }

    throw new Error("All models failed");
  }

  // Express HTTP server
  const app = express();
  app.use(express.json());

  // POST /prompt — send prompt, returns on completion
  app.post("/prompt", async (req, res) => {
    const { prompt } = req.body as PromptRequest;
    if (!prompt) {
      res.status(400).json({ error: "prompt is required" });
      return;
    }

    if (busy) {
      res.status(429).json({ error: "Agent is busy" });
      return;
    }

    busy = true;
    try {
      const result = await handlePrompt(prompt);
      lastOutput = result.output;
      totalToolCalls += result.toolCalls;
      totalTokensUsed += result.tokensUsed;
      totalCost += result.cost;
      res.json(result);
    } catch (err: any) {
      res.status(500).json({ error: err.message });
    } finally {
      busy = false;
    }
  });

  // POST /prompt/stream — SSE event stream
  app.post("/prompt/stream", async (req, res) => {
    const { prompt } = req.body as PromptRequest;
    if (!prompt) {
      res.status(400).json({ error: "prompt is required" });
      return;
    }

    if (busy) {
      res.status(429).json({ error: "Agent is busy" });
      return;
    }

    res.setHeader("Content-Type", "text/event-stream");
    res.setHeader("Cache-Control", "no-cache");
    res.setHeader("Connection", "keep-alive");

    busy = true;
    try {
      // Stream events from Pi session
      const result = await handlePrompt(prompt);
      res.write(`data: ${JSON.stringify({ type: "complete", ...result })}\n\n`);
      lastOutput = result.output;
    } catch (err: any) {
      res.write(`data: ${JSON.stringify({ type: "error", error: err.message })}\n\n`);
    } finally {
      busy = false;
      res.end();
    }
  });

  // POST /steer — redirect agent mid-conversation
  app.post("/steer", async (req, res) => {
    const { message } = req.body as SteerRequest;
    if (!message) {
      res.status(400).json({ error: "message is required" });
      return;
    }

    try {
      await session.steer(message);
      res.json({ ok: true });
    } catch (err: any) {
      res.status(500).json({ error: err.message });
    }
  });

  // POST /followup — continue in existing session
  app.post("/followup", async (req, res) => {
    const { prompt } = req.body as FollowupRequest;
    if (!prompt) {
      res.status(400).json({ error: "prompt is required" });
      return;
    }

    if (busy) {
      res.status(429).json({ error: "Agent is busy" });
      return;
    }

    busy = true;
    try {
      const result = await session.followUp(prompt);
      const output = typeof result === "string" ? result : JSON.stringify(result);
      lastOutput = output;
      res.json({ output, model: activeModel });
    } catch (err: any) {
      res.status(500).json({ error: err.message });
    } finally {
      busy = false;
    }
  });

  // DELETE /abort — cancel current execution
  app.delete("/abort", async (_req, res) => {
    try {
      await session.abort();
      busy = false;
      res.json({ ok: true });
    } catch (err: any) {
      res.status(500).json({ error: err.message });
    }
  });

  // GET /healthz — health check
  app.get("/healthz", (_req, res) => {
    res.json({ status: "ok" });
  });

  // GET /status — session stats
  app.get("/status", (_req, res) => {
    const status: AgentStatus = {
      busy,
      output: lastOutput,
      toolCalls: totalToolCalls,
      tokensUsed: totalTokensUsed,
      cost: totalCost.toFixed(4),
      model: activeModel,
    };
    res.json(status);
  });

  app.listen(PORT, () => {
    console.log(`[agent-server] Listening on :${PORT}`);
  });
}

main().catch((err) => {
  console.error("[agent-server] Fatal error:", err);
  process.exit(1);
});
