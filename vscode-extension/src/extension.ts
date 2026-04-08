import * as vscode from 'vscode';
import { BarqWitnessDecorationProvider } from './provider';
import { runReport } from './runner';
import { Report, Segment } from './types';

let decorationProvider: BarqWitnessDecorationProvider | undefined;
let autoRefreshHandle: NodeJS.Timeout | undefined;
let currentReport: Report | undefined;
let panelRef: vscode.WebviewPanel | undefined;

function getConfig(): {
  binaryPath: string;
  topN: number;
  autoRefreshSeconds: number;
} {
  const cfg = vscode.workspace.getConfiguration('barq-witness');
  return {
    binaryPath: cfg.get<string>('binaryPath', 'barq-witness'),
    topN: cfg.get<number>('topN', 10),
    autoRefreshSeconds: cfg.get<number>('autoRefreshSeconds', 0),
  };
}

function getWorkspaceRoot(): string | undefined {
  const folders = vscode.workspace.workspaceFolders;
  if (!folders || folders.length === 0) {
    return undefined;
  }
  return folders[0].uri.fsPath;
}

async function refresh(context: vscode.ExtensionContext): Promise<void> {
  const root = getWorkspaceRoot();
  if (!root) {
    vscode.window.showWarningMessage('barq-witness: no workspace folder open');
    return;
  }

  const { binaryPath, topN } = getConfig();

  try {
    currentReport = await runReport(root, binaryPath, topN);
  } catch (err) {
    vscode.window.showErrorMessage(`barq-witness: ${String(err)}`);
    return;
  }

  applyDecorationsToVisible(currentReport);

  if (panelRef) {
    panelRef.webview.html = buildPanelHtml(currentReport);
  }

  const total = currentReport.total_segments;
  if (total === 0) {
    vscode.window.setStatusBarMessage('barq-witness: no flagged segments', 4000);
  } else {
    vscode.window.setStatusBarMessage(
      `barq-witness: ${total} segment(s) -- T1:${currentReport.tier1_count} T2:${currentReport.tier2_count} T3:${currentReport.tier3_count}`,
      6000
    );
  }
}

function applyDecorationsToVisible(report: Report): void {
  if (!decorationProvider) {
    return;
  }
  for (const editor of vscode.window.visibleTextEditors) {
    decorationProvider.applyDecorations(editor, report.segments);
  }
}

function buildPanelHtml(report: Report): string {
  const date = new Date(report.generated_at).toLocaleString();
  const rows = (report.segments ?? [])
    .map((seg: Segment) => {
      const tierClass = `tier${seg.tier}`;
      const codes = seg.reason_codes.join(', ') || '--';
      const expl = escapeHtml(seg.explanation ?? '');
      return `<tr class="${tierClass}">
        <td>${escapeHtml(seg.file_path)}</td>
        <td>${seg.line_start}--${seg.line_end}</td>
        <td>${seg.tier}</td>
        <td>${seg.score.toFixed(2)}</td>
        <td>${escapeHtml(codes)}</td>
        <td>${expl}</td>
      </tr>`;
    })
    .join('\n');

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>barq-witness Attention Map</title>
  <style>
    body { font-family: var(--vscode-font-family, sans-serif); font-size: var(--vscode-font-size, 13px); color: var(--vscode-foreground); background: var(--vscode-editor-background); padding: 12px; }
    h2 { margin-top: 0; }
    .meta { margin-bottom: 12px; color: var(--vscode-descriptionForeground); font-size: 0.9em; }
    table { border-collapse: collapse; width: 100%; }
    th, td { text-align: left; padding: 4px 8px; border-bottom: 1px solid var(--vscode-panel-border, #333); }
    th { background: var(--vscode-editor-lineHighlightBackground); font-weight: 600; }
    .tier1 { background: rgba(220, 38, 38, 0.12); }
    .tier2 { background: rgba(234, 179, 8, 0.10); }
    .tier3 { background: rgba(59, 130, 246, 0.08); }
    .empty { padding: 16px; color: var(--vscode-descriptionForeground); }
  </style>
</head>
<body>
  <h2>barq-witness Attention Map</h2>
  <div class="meta">
    Generated: ${escapeHtml(date)} &nbsp;|&nbsp;
    Range: ${escapeHtml(report.commit_range)} &nbsp;|&nbsp;
    Segments: ${report.total_segments}
    (T1: ${report.tier1_count}, T2: ${report.tier2_count}, T3: ${report.tier3_count})
  </div>
  ${
    rows
      ? `<table>
    <thead><tr><th>File</th><th>Lines</th><th>Tier</th><th>Score</th><th>Reason Codes</th><th>Explanation</th></tr></thead>
    <tbody>${rows}</tbody>
  </table>`
      : '<div class="empty">No flagged segments found for this commit range.</div>'
  }
</body>
</html>`;
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function startAutoRefresh(context: vscode.ExtensionContext, intervalSeconds: number): void {
  stopAutoRefresh();
  if (intervalSeconds <= 0) {
    return;
  }
  autoRefreshHandle = setInterval(() => {
    refresh(context).catch(() => {
      // Silently ignore auto-refresh errors to avoid notification spam.
    });
  }, intervalSeconds * 1000);
}

function stopAutoRefresh(): void {
  if (autoRefreshHandle !== undefined) {
    clearInterval(autoRefreshHandle);
    autoRefreshHandle = undefined;
  }
}

export function activate(context: vscode.ExtensionContext): void {
  decorationProvider = new BarqWitnessDecorationProvider();

  // Re-apply decorations when a new editor becomes visible.
  context.subscriptions.push(
    vscode.window.onDidChangeVisibleTextEditors(() => {
      if (currentReport) {
        applyDecorationsToVisible(currentReport);
      }
    })
  );

  // Re-apply decorations when active editor changes.
  context.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor((editor) => {
      if (editor && currentReport) {
        decorationProvider?.applyDecorations(editor, currentReport.segments);
      }
    })
  );

  // Command: refresh
  const refreshCmd = vscode.commands.registerCommand('barq-witness.refresh', () => {
    refresh(context).catch((err: unknown) => {
      vscode.window.showErrorMessage(`barq-witness refresh failed: ${String(err)}`);
    });
  });

  // Command: showPanel
  const showPanelCmd = vscode.commands.registerCommand('barq-witness.showPanel', () => {
    if (panelRef) {
      panelRef.reveal(vscode.ViewColumn.Two);
      return;
    }

    panelRef = vscode.window.createWebviewPanel(
      'barqWitnessPanel',
      'barq-witness Attention Map',
      vscode.ViewColumn.Two,
      { enableScripts: false }
    );

    panelRef.webview.html = currentReport
      ? buildPanelHtml(currentReport)
      : buildPanelHtml({
          commit_range: '--',
          generated_at: Date.now(),
          total_segments: 0,
          tier1_count: 0,
          tier2_count: 0,
          tier3_count: 0,
          segments: [],
        });

    panelRef.onDidDispose(() => {
      panelRef = undefined;
    });

    context.subscriptions.push(panelRef);
  });

  // Command: clearDecorations
  const clearCmd = vscode.commands.registerCommand('barq-witness.clearDecorations', () => {
    currentReport = undefined;
    decorationProvider?.clearAll();
    vscode.window.setStatusBarMessage('barq-witness: decorations cleared', 3000);
  });

  context.subscriptions.push(refreshCmd, showPanelCmd, clearCmd);
  context.subscriptions.push({ dispose: () => decorationProvider?.dispose() });
  context.subscriptions.push({ dispose: () => stopAutoRefresh() });

  // Start auto-refresh if configured.
  const { autoRefreshSeconds } = getConfig();
  startAutoRefresh(context, autoRefreshSeconds);

  // Watch for configuration changes.
  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration('barq-witness.autoRefreshSeconds')) {
        const { autoRefreshSeconds: newInterval } = getConfig();
        startAutoRefresh(context, newInterval);
      }
    })
  );

  // Trigger an initial refresh in the background.
  refresh(context).catch(() => {
    // Ignore initial refresh failures -- trace.db may not exist yet.
  });
}

export function deactivate(): void {
  stopAutoRefresh();
  decorationProvider?.dispose();
  decorationProvider = undefined;
  panelRef?.dispose();
  panelRef = undefined;
}
