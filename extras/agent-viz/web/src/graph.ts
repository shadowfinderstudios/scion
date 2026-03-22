import ForceGraph from 'force-graph';
import type { FileNode, GraphNode, GraphLink } from './types';

type ForceGraphInstance = InstanceType<typeof ForceGraph>;

const DIR_RADIUS = 6;
const FILE_RADIUS = 3;
const HIGHLIGHT_DURATION = 3000; // ms

export class FileGraph {
  private graph: ForceGraphInstance;
  private nodes: Map<string, GraphNode> = new Map();
  private container: HTMLElement;

  constructor(container: HTMLElement) {
    this.container = container;
    this.graph = new ForceGraph(container)
      .nodeId('id')
      .nodeLabel('name')
      .nodeCanvasObject((node, ctx, globalScale) => this.drawNode(node as GraphNode, ctx, globalScale))
      .nodePointerAreaPaint((node, color, ctx) => this.drawNodeArea(node as GraphNode, color, ctx))
      .linkColor(() => 'rgba(255,255,255,0.15)')
      .linkWidth(1)
      .d3AlphaDecay(0.02)
      .d3VelocityDecay(0.3)
      .cooldownTicks(200)
      .warmupTicks(100)
      .backgroundColor('transparent');
  }

  getGraph(): InstanceType<typeof ForceGraph> {
    return this.graph;
  }

  init(files: FileNode[]): void {
    const graphNodes: GraphNode[] = [];
    const links: GraphLink[] = [];

    for (const f of files) {
      const node: GraphNode = {
        id: f.id,
        name: f.name,
        isDir: f.isDir,
        parent: f.parent,
        highlighted: false,
      };
      this.nodes.set(f.id, node);
      graphNodes.push(node);
    }

    // Create links from parent-child relationships
    for (const f of files) {
      if (f.parent && f.parent !== '.' && this.nodes.has(f.parent)) {
        links.push({ source: f.parent, target: f.id });
      }
    }

    this.graph.graphData({ nodes: graphNodes, links });

    // Center on graph after layout settles
    setTimeout(() => {
      this.graph.zoomToFit(400, 80);
    }, 2000);
  }

  highlightFile(filePath: string): void {
    const node = this.nodes.get(filePath);
    if (node) {
      node.highlighted = true;
      node.highlightTime = Date.now();
    }
    // Also highlight parent directories
    let parent = this.getParentPath(filePath);
    while (parent && parent !== '.') {
      const pNode = this.nodes.get(parent);
      if (pNode) {
        pNode.highlighted = true;
        pNode.highlightTime = Date.now();
      }
      parent = this.getParentPath(parent);
    }
  }

  getNodePosition(nodeId: string): { x: number; y: number } | null {
    const node = this.nodes.get(nodeId);
    if (node && node.x !== undefined && node.y !== undefined) {
      return { x: node.x, y: node.y };
    }
    return null;
  }

  resize(width: number, height: number): void {
    this.graph.width(width).height(height);
  }

  private drawNode(node: GraphNode, ctx: CanvasRenderingContext2D, globalScale: number): void {
    const r = node.isDir ? DIR_RADIUS : FILE_RADIUS;
    const x = node.x ?? 0;
    const y = node.y ?? 0;

    // Check highlight fade
    let alpha = 1;
    let glowing = false;
    if (node.highlighted && node.highlightTime) {
      const elapsed = Date.now() - node.highlightTime;
      if (elapsed > HIGHLIGHT_DURATION) {
        node.highlighted = false;
      } else {
        glowing = true;
        alpha = 1 - elapsed / HIGHLIGHT_DURATION;
      }
    }

    // Glow effect
    if (glowing) {
      ctx.save();
      ctx.globalAlpha = alpha * 0.6;
      ctx.shadowBlur = 15;
      ctx.shadowColor = '#4fc3f7';
      ctx.beginPath();
      ctx.arc(x, y, r + 4, 0, Math.PI * 2);
      ctx.fillStyle = '#4fc3f7';
      ctx.fill();
      ctx.restore();
    }

    // Node circle
    ctx.beginPath();
    ctx.arc(x, y, r, 0, Math.PI * 2);
    if (node.isDir) {
      ctx.fillStyle = glowing ? '#4fc3f7' : '#546e7a';
    } else {
      ctx.fillStyle = glowing ? '#81d4fa' : '#78909c';
    }
    ctx.fill();

    // Border
    ctx.strokeStyle = 'rgba(255,255,255,0.3)';
    ctx.lineWidth = 0.5;
    ctx.stroke();

    // Label (only when zoomed in enough)
    if (globalScale > 1.5) {
      ctx.font = `${Math.max(3, 10 / globalScale)}px sans-serif`;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'top';
      ctx.fillStyle = 'rgba(255,255,255,0.7)';
      ctx.fillText(node.name, x, y + r + 2);
    }
  }

  private drawNodeArea(node: GraphNode, color: string, ctx: CanvasRenderingContext2D): void {
    const r = node.isDir ? DIR_RADIUS : FILE_RADIUS;
    ctx.beginPath();
    ctx.arc(node.x ?? 0, node.y ?? 0, r + 2, 0, Math.PI * 2);
    ctx.fillStyle = color;
    ctx.fill();
  }

  private getParentPath(path: string): string {
    const idx = path.lastIndexOf('/');
    if (idx <= 0) return '.';
    return path.substring(0, idx);
  }
}
