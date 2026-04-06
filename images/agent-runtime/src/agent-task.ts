/**
 * Agent Runtime — Task Mode (agent-task.ts)
 *
 * Reads AGENT_PROMPT env var, runs the prompt through Pi SDK,
 * writes a structured JSON result to stdout, then exits.
 * Uses SessionManager.inMemory() — nothing persists.
 */

import { readFileSync } from "node:fs";
import { createAgentSession } from "@mariozechner/pi-coding-agent";
import {
  SessionManager,
  AuthStorage,
  ModelRegistry,
} from "@mariozechner/pi-agent-core";
import type { OperatorConfig, TaskResult } from "./types.js";
import { loadOperatorExtension } from "./operator-extension.js";

const CONFIG_PATH = "/etc/operator/config.json";

async function main() {
  const prompt = process.env.AGENT_PROMPT;
  if (!prompt) {
    const result: TaskResult = {
      output: "",
      toolCalls: 0,
      tokensUsed: 0,
      cost: 0,
      model: "",
      success: false,
      error: "AGENT_PROMPT environment variable is not set",
    };
    process.stdout.write(JSON.stringify(result) + "\n");
    process.exit(1);
  }

  // Load operator config
  const config: OperatorConfig = JSON.parse(
    readFileSync(CONFIG_PATH, "utf-8")
  );

  console.error(`[agent-task] Running task with model: ${config.primaryModel}`);
  console.error(`[agent-task] Prompt: ${prompt.substring(0, 100)}...`);

  // Setup Pi SDK (in-memory, no persistence)
  const authStorage = AuthStorage.create();
  const modelRegistry = ModelRegistry.create(authStorage);

  // Register provider API keys
  for (const provider of config.providers) {
    const envKey = `${provider.name.toUpperCase()}_API_KEY`;
    const apiKey = process.env[envKey];
    if (apiKey) {
      authStorage.set(provider.name, apiKey);
    }
  }

  const session = await createAgentSession({
    sessionManager: SessionManager.inMemory(),
    authStorage,
    modelRegistry,
  });

  // Load operator extension
  await loadOperatorExtension(session, config);

  // Execute prompt with fallback
  const models = [config.primaryModel, ...(config.fallbackModels ?? [])];
  let lastError: Error | null = null;

  for (const model of models) {
    try {
      session.setModel(model);
      const rawResult = await session.prompt(prompt);
      const output = typeof rawResult === "string" ? rawResult : JSON.stringify(rawResult);

      const result: TaskResult = {
        output,
        toolCalls: 0,
        tokensUsed: 0,
        cost: 0,
        model,
        success: true,
      };

      // Write structured result to stdout (operator parses this)
      process.stdout.write(JSON.stringify(result) + "\n");
      process.exit(0);
    } catch (err: any) {
      lastError = err;
      const isRetryable = err?.status === 429 || err?.status >= 500;
      if (isRetryable && model !== models[models.length - 1]) {
        console.error(`[agent-task] Model ${model} failed, trying fallback...`);
        continue;
      }
    }
  }

  // All models failed
  const result: TaskResult = {
    output: "",
    toolCalls: 0,
    tokensUsed: 0,
    cost: 0,
    model: config.primaryModel,
    success: false,
    error: lastError?.message ?? "Unknown error",
  };

  process.stdout.write(JSON.stringify(result) + "\n");
  process.exit(1);
}

main().catch((err) => {
  console.error("[agent-task] Fatal error:", err);
  process.exit(1);
});
