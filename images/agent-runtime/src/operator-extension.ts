/**
 * Operator Extension — loaded into every agent pod.
 *
 * Reads config from /etc/operator/config.json and:
 * 1. Loads OCI/ConfigMap/inline tools from /tools/
 * 2. Registers run_agent + get_agent_run tools
 * 3. Configures pi-mcp-adapter for MCP bindings
 * 4. Registers beforeToolCall/afterToolCall security hooks
 * 5. Emits telemetry events
 */

import { readFileSync, existsSync } from "node:fs";
import type { OperatorConfig } from "./types.js";
import { K8sClient } from "./k8s-client.js";

// Pi extension API types (from Pi SDK)
interface ExtensionAPI {
  registerTool(tool: ToolDefinition): void;
  on(event: string, handler: (data: any) => Promise<void>): void;
  events: {
    emit(event: string, data: any): void;
  };
  agent: {
    beforeToolCall?: (toolName: string, params: any) => Promise<void>;
    afterToolCall?: (toolName: string, params: any, result: any) => Promise<void>;
  };
}

interface ToolDefinition {
  name: string;
  description: string;
  parameters: any;
  execute: (id: string, params: any) => Promise<ToolResult>;
}

interface ToolResult {
  content: Array<{ type: string; text: string }>;
  details?: any;
}

/**
 * Load the operator extension into a Pi session.
 */
export async function loadOperatorExtension(
  pi: ExtensionAPI,
  config: OperatorConfig
): Promise<void> {
  const k8sClient = new K8sClient();

  // 1. Load tools from /tools/ directories
  for (const tool of config.tools) {
    try {
      const indexPath = `${tool.path}/index.js`;
      if (existsSync(indexPath)) {
        const mod = await import(indexPath);
        if (mod.tools && Array.isArray(mod.tools)) {
          for (const t of mod.tools) {
            pi.registerTool(t);
          }
          console.log(`[extension] Loaded ${mod.tools.length} tools from ${tool.name}`);
        }
      }
    } catch (err) {
      console.error(`[extension] Failed to load tool ${tool.name}:`, err);
    }
  }

  // 2. Register run_agent tool (agentic orchestration)
  pi.registerTool({
    name: "run_agent",
    description:
      "Trigger another agent with a prompt. Creates an AgentRun tracked by the operator.",
    parameters: {
      type: "object",
      properties: {
        agent: { type: "string", description: "Agent name" },
        prompt: { type: "string", description: "Prompt to send" },
      },
      required: ["agent", "prompt"],
    },
    async execute(_id: string, params: { agent: string; prompt: string }) {
      const run = await k8sClient.createAgentRun({
        agentRef: params.agent,
        prompt: params.prompt,
        source: "agent",
        sourceRef: process.env.AGENT_NAME ?? "unknown",
      });
      return {
        content: [{ type: "text", text: `AgentRun ${run.name} created` }],
      };
    },
  });

  // 3. Register get_agent_run tool
  pi.registerTool({
    name: "get_agent_run",
    description: "Check the status and output of an AgentRun.",
    parameters: {
      type: "object",
      properties: {
        name: { type: "string", description: "AgentRun name" },
      },
      required: ["name"],
    },
    async execute(_id: string, params: { name: string }) {
      const run = await k8sClient.getAgentRun(params.name);
      return {
        content: [
          {
            type: "text",
            text: `Phase: ${run.status?.phase ?? "Unknown"}\nOutput: ${run.status?.output ?? "(pending)"}`,
          },
        ],
        details: {
          phase: run.status?.phase,
          output: run.status?.output,
        },
      };
    },
  });

  // 4. Configure pi-mcp-adapter for MCP bindings
  if (config.mcpServers && config.mcpServers.length > 0) {
    try {
      const { createMcpAdapter } = await import("pi-mcp-adapter");
      const adapter = await createMcpAdapter({
        configPath: "/etc/mcp/mcp.json",
        directTools: config.mcpServers.flatMap((s) => s.directTools ?? []),
      });
      adapter.registerTools(pi);
      console.log(
        `[extension] MCP adapter configured for ${config.mcpServers.length} server(s)`
      );
    } catch (err) {
      console.error("[extension] Failed to configure MCP adapter:", err);
    }
  }

  // 5. beforeToolCall / afterToolCall hooks — defense-in-depth
  if (config.toolHooks) {
    pi.agent.beforeToolCall = async (
      toolName: string,
      params: any
    ): Promise<void> => {
      // Block dangerous patterns in bash commands
      if (
        toolName === "bash" &&
        config.toolHooks?.blockedCommands?.length
      ) {
        const cmd = params.command || "";
        for (const blocked of config.toolHooks.blockedCommands) {
          if (cmd.includes(blocked)) {
            throw new Error(`Blocked command pattern: ${blocked}`);
          }
        }
      }

      // Path-based access control
      if (config.toolHooks?.allowedPaths?.length) {
        const path = params.path || params.filePath || "";
        if (
          path &&
          !config.toolHooks.allowedPaths.some((p: string) =>
            path.startsWith(p)
          )
        ) {
          throw new Error(`Path not in allowed list: ${path}`);
        }
      }
    };

    pi.agent.afterToolCall = async (
      toolName: string,
      params: any,
      _result: any
    ): Promise<void> => {
      // Audit logging for sensitive tools
      if (config.toolHooks?.auditTools?.includes(toolName)) {
        pi.events.emit("telemetry:audit", {
          tool: toolName,
          params,
          timestamp: Date.now(),
        });
      }
    };
  }

  // 6. Telemetry events
  pi.on("tool_execution_end", async (event: any) => {
    pi.events.emit("telemetry:tool", {
      tool: event.toolName,
      success: !event.isError,
      ts: Date.now(),
    });
  });

  pi.on("agent_end", async (event: any) => {
    for (const msg of event.messages ?? []) {
      if (msg.role === "assistant" && msg.usage) {
        pi.events.emit("telemetry:usage", {
          model: msg.model,
          tokens: msg.usage,
          cost: msg.usage.cost?.total,
        });
      }
    }
  });

  console.log("[extension] Operator extension loaded");
}
