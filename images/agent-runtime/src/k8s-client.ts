/**
 * Minimal Kubernetes client for the operator extension.
 * Uses @kubernetes/client-node for in-cluster communication.
 * Creates/gets AgentRun CRs from within agent pods.
 */

import * as k8s from "@kubernetes/client-node";

const API_GROUP = "agents.agenticops.io";
const API_VERSION = "v1alpha1";
const AGENT_RUN_PLURAL = "agentruns";

interface AgentRunSpec {
  agentRef: string;
  prompt: string;
  source: string;
  sourceRef: string;
}

interface AgentRunCR {
  metadata: { name: string; namespace: string };
  spec: AgentRunSpec;
  status?: {
    phase?: string;
    output?: string;
    toolCalls?: number;
    tokensUsed?: number;
    cost?: string;
    model?: string;
  };
}

export class K8sClient {
  private customApi: k8s.CustomObjectsApi;
  private namespace: string;

  constructor() {
    const kc = new k8s.KubeConfig();
    kc.loadFromCluster();
    this.customApi = kc.makeApiClient(k8s.CustomObjectsApi);
    this.namespace = process.env.AGENT_NAMESPACE ?? "default";
  }

  /**
   * Create an AgentRun CR.
   */
  async createAgentRun(spec: AgentRunSpec): Promise<{ name: string }> {
    const name = `${spec.agentRef}-run-${Date.now()}`;

    const body = {
      apiVersion: `${API_GROUP}/${API_VERSION}`,
      kind: "AgentRun",
      metadata: {
        name,
        namespace: this.namespace,
        labels: {
          "agents.agenticops.io/agent": spec.agentRef,
        },
      },
      spec,
    };

    await this.customApi.createNamespacedCustomObject({
      group: API_GROUP,
      version: API_VERSION,
      namespace: this.namespace,
      plural: AGENT_RUN_PLURAL,
      body,
    });

    return { name };
  }

  /**
   * Get an AgentRun CR by name.
   */
  async getAgentRun(name: string): Promise<AgentRunCR> {
    const response = await this.customApi.getNamespacedCustomObject({
      group: API_GROUP,
      version: API_VERSION,
      namespace: this.namespace,
      plural: AGENT_RUN_PLURAL,
      name,
    });

    return response as unknown as AgentRunCR;
  }
}
