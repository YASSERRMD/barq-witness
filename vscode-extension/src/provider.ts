import * as vscode from 'vscode';
import { Segment } from './types';

function tierLabel(tier: number): string {
  switch (tier) {
    case 1: return 'Tier 1 -- Critical';
    case 2: return 'Tier 2 -- High';
    case 3: return 'Tier 3 -- Medium';
    default: return `Tier ${tier}`;
  }
}

export class BarqWitnessDecorationProvider {
  private readonly tier1Decoration: vscode.TextEditorDecorationType;
  private readonly tier2Decoration: vscode.TextEditorDecorationType;
  private readonly tier3Decoration: vscode.TextEditorDecorationType;

  constructor() {
    // Tier 1: red gutter icon + semi-transparent red background
    this.tier1Decoration = vscode.window.createTextEditorDecorationType({
      backgroundColor: 'rgba(220, 38, 38, 0.15)',
      overviewRulerColor: 'rgba(220, 38, 38, 0.8)',
      overviewRulerLane: vscode.OverviewRulerLane.Right,
      gutterIconSize: 'contain',
      before: {
        contentText: '\u25CF',
        color: 'rgba(220, 38, 38, 0.9)',
        margin: '0 4px 0 0',
      },
      isWholeLine: true,
    });

    // Tier 2: yellow/orange gutter icon + yellow background
    this.tier2Decoration = vscode.window.createTextEditorDecorationType({
      backgroundColor: 'rgba(234, 179, 8, 0.12)',
      overviewRulerColor: 'rgba(234, 179, 8, 0.8)',
      overviewRulerLane: vscode.OverviewRulerLane.Right,
      gutterIconSize: 'contain',
      before: {
        contentText: '\u25CF',
        color: 'rgba(234, 179, 8, 0.9)',
        margin: '0 4px 0 0',
      },
      isWholeLine: true,
    });

    // Tier 3: blue/dim gutter icon
    this.tier3Decoration = vscode.window.createTextEditorDecorationType({
      overviewRulerColor: 'rgba(59, 130, 246, 0.6)',
      overviewRulerLane: vscode.OverviewRulerLane.Right,
      before: {
        contentText: '\u25CB',
        color: 'rgba(59, 130, 246, 0.7)',
        margin: '0 4px 0 0',
      },
      isWholeLine: true,
    });
  }

  public applyDecorations(editor: vscode.TextEditor, segments: Segment[]): void {
    const filePath = vscode.workspace.asRelativePath(editor.document.uri, false);
    const absPath = editor.document.uri.fsPath;

    const tier1: vscode.DecorationOptions[] = [];
    const tier2: vscode.DecorationOptions[] = [];
    const tier3: vscode.DecorationOptions[] = [];

    for (const seg of segments) {
      // Match by relative path or absolute path.
      if (seg.file_path !== filePath && seg.file_path !== absPath) {
        continue;
      }

      // VS Code lines are 0-based; segment lines are 1-based.
      const startLine = Math.max(0, seg.line_start - 1);
      const endLine = Math.max(startLine, seg.line_end - 1);

      const startPos = new vscode.Position(startLine, 0);
      const endPos = editor.document.lineAt(Math.min(endLine, editor.document.lineCount - 1)).range.end;
      const range = new vscode.Range(startPos, endPos);

      const hoverLines: string[] = [
        `**barq-witness** -- ${tierLabel(seg.tier)}`,
        ``,
        `Score: ${seg.score.toFixed(2)}`,
        `Lines: ${seg.line_start}--${seg.line_end}`,
        `Reason codes: ${seg.reason_codes.length > 0 ? seg.reason_codes.join(', ') : 'none'}`,
      ];

      if (seg.explanation) {
        hoverLines.push(``, `**Explanation:** ${seg.explanation}`);
      }

      if (seg.prompt_text) {
        const truncated =
          seg.prompt_text.length > 120 ? seg.prompt_text.slice(0, 120) + '...' : seg.prompt_text;
        hoverLines.push(``, `**Prompt:** ${truncated}`);
      }

      hoverLines.push(``, `Accepted in: ${seg.accepted_sec >= 0 ? seg.accepted_sec + 's' : 'unknown'}`);
      hoverLines.push(`Executed after: ${seg.executed ? 'yes' : 'no'}`);

      const hoverMessage = new vscode.MarkdownString(hoverLines.join('\n'));
      hoverMessage.isTrusted = false;

      const decoration: vscode.DecorationOptions = { range, hoverMessage };

      switch (seg.tier) {
        case 1:
          tier1.push(decoration);
          break;
        case 2:
          tier2.push(decoration);
          break;
        default:
          tier3.push(decoration);
      }
    }

    editor.setDecorations(this.tier1Decoration, tier1);
    editor.setDecorations(this.tier2Decoration, tier2);
    editor.setDecorations(this.tier3Decoration, tier3);
  }

  public clearAll(): void {
    for (const editor of vscode.window.visibleTextEditors) {
      editor.setDecorations(this.tier1Decoration, []);
      editor.setDecorations(this.tier2Decoration, []);
      editor.setDecorations(this.tier3Decoration, []);
    }
  }

  public dispose(): void {
    this.tier1Decoration.dispose();
    this.tier2Decoration.dispose();
    this.tier3Decoration.dispose();
  }
}
