import type { AgentInfo, AgentRenderState, AgentStateEvent } from './types';
import { getIconForState } from './icons';

const AGENT_RADIUS = 24;
const RING_PADDING = 100; // distance from file graph bounding box

export class AgentRing {
  private agents: Map<string, AgentRenderState> = new Map();
  private ringRadius = 300;
  private centerX = 0;
  private centerY = 0;
  private animationPhase = 0;

  init(agentInfos: AgentInfo[], centerX: number, centerY: number): void {
    this.centerX = centerX;
    this.centerY = centerY;
    this.ringRadius = Math.min(centerX, centerY) - RING_PADDING;
    if (this.ringRadius < 150) this.ringRadius = 150;

    const n = agentInfos.length;
    agentInfos.forEach((info, i) => {
      const angle = (2 * Math.PI * i) / n - Math.PI / 2; // start from top
      this.agents.set(info.id, {
        info,
        angle,
        x: this.centerX + Math.cos(angle) * this.ringRadius,
        y: this.centerY + Math.sin(angle) * this.ringRadius,
        phase: 'created',
        activity: 'idle',
        toolName: '',
      });
    });
  }

  updateLayout(centerX: number, centerY: number): void {
    this.centerX = centerX;
    this.centerY = centerY;
    this.ringRadius = Math.min(centerX, centerY) - RING_PADDING;
    if (this.ringRadius < 150) this.ringRadius = 150;

    for (const agent of this.agents.values()) {
      agent.x = this.centerX + Math.cos(agent.angle) * this.ringRadius;
      agent.y = this.centerY + Math.sin(agent.angle) * this.ringRadius;
    }
  }

  updateState(event: AgentStateEvent): void {
    // Find agent by ID or by name match
    let agent = this.agents.get(event.agentId);
    if (!agent) {
      // Try finding by name in the agent list
      for (const a of this.agents.values()) {
        if (a.info.id === event.agentId) {
          agent = a;
          break;
        }
      }
    }
    if (!agent) return;

    if (event.phase) agent.phase = event.phase;
    if (event.activity) agent.activity = event.activity;
    if (event.toolName !== undefined) agent.toolName = event.toolName;
  }

  getAgentPosition(agentIdOrName: string): { x: number; y: number } | null {
    // Try by ID first
    const byId = this.agents.get(agentIdOrName);
    if (byId) return { x: byId.x, y: byId.y };
    // Try by name
    for (const a of this.agents.values()) {
      if (a.info.name === agentIdOrName) return { x: a.x, y: a.y };
    }
    return null;
  }

  getAgentColor(agentIdOrName: string): string {
    const byId = this.agents.get(agentIdOrName);
    if (byId) return byId.info.color;
    for (const a of this.agents.values()) {
      if (a.info.name === agentIdOrName) return a.info.color;
    }
    return '#888';
  }

  draw(ctx: CanvasRenderingContext2D): void {
    this.animationPhase = (Date.now() / 1000) % (2 * Math.PI);

    // Draw ring circle (faint)
    ctx.beginPath();
    ctx.arc(this.centerX, this.centerY, this.ringRadius, 0, Math.PI * 2);
    ctx.strokeStyle = 'rgba(255,255,255,0.05)';
    ctx.lineWidth = 1;
    ctx.stroke();

    // Draw each agent
    for (const agent of this.agents.values()) {
      this.drawAgent(ctx, agent);
    }
  }

  private drawAgent(ctx: CanvasRenderingContext2D, agent: AgentRenderState): void {
    const { x, y, info, phase, activity } = agent;
    const icon = getIconForState(phase, activity);

    // Pulse effect for pulsing states
    let pulseScale = 1;
    if (icon.pulse) {
      pulseScale = 1 + 0.08 * Math.sin(this.animationPhase * 3);
    }

    const r = AGENT_RADIUS * pulseScale;

    // Outer glow for active agents
    if (activity === 'thinking' || activity === 'executing') {
      ctx.save();
      ctx.globalAlpha = 0.3 + 0.1 * Math.sin(this.animationPhase * 3);
      ctx.shadowBlur = 20;
      ctx.shadowColor = icon.color;
      ctx.beginPath();
      ctx.arc(x, y, r + 6, 0, Math.PI * 2);
      ctx.fillStyle = icon.color;
      ctx.fill();
      ctx.restore();
    }

    // Agent circle background
    ctx.beginPath();
    ctx.arc(x, y, r, 0, Math.PI * 2);
    ctx.fillStyle = info.color;
    ctx.fill();

    // Agent circle border
    ctx.strokeStyle = icon.color;
    ctx.lineWidth = 2.5;
    ctx.stroke();

    // Icon inside circle (draw a smaller circle with icon color as indicator)
    ctx.beginPath();
    ctx.arc(x, y, r * 0.45, 0, Math.PI * 2);
    ctx.fillStyle = icon.color;
    ctx.fill();

    // Agent name label below
    ctx.font = 'bold 12px sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'top';
    ctx.fillStyle = 'rgba(255,255,255,0.9)';
    ctx.fillText(info.name, x, y + r + 6);

    // Tool name (when executing)
    if (activity === 'executing' && agent.toolName) {
      ctx.font = '10px sans-serif';
      ctx.fillStyle = 'rgba(255,255,255,0.5)';
      ctx.fillText(agent.toolName, x, y + r + 20);
    }
  }
}
