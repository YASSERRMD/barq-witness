import * as cp from 'child_process';
import { Report } from './types';

export async function runReport(
  workspaceRoot: string,
  binaryPath: string,
  topN: number
): Promise<Report> {
  return new Promise((resolve, reject) => {
    const args = ['report', '--format', 'json', '--top', String(topN)];
    const proc = cp.spawn(binaryPath, args, {
      cwd: workspaceRoot,
      env: { ...process.env, CLAUDE_PROJECT_DIR: workspaceRoot },
    });

    let stdout = '';
    let stderr = '';

    proc.stdout.on('data', (chunk: Buffer) => {
      stdout += chunk.toString();
    });

    proc.stderr.on('data', (chunk: Buffer) => {
      stderr += chunk.toString();
    });

    proc.on('error', (err: Error) => {
      reject(new Error(`failed to spawn barq-witness: ${err.message}`));
    });

    proc.on('close', (code: number | null) => {
      if (code !== 0) {
        reject(
          new Error(
            `barq-witness exited with code ${code ?? 'null'}${stderr ? ': ' + stderr.trim() : ''}`
          )
        );
        return;
      }

      const trimmed = stdout.trim();
      if (!trimmed) {
        reject(new Error('barq-witness produced no output'));
        return;
      }

      let report: Report;
      try {
        report = JSON.parse(trimmed) as Report;
      } catch (e) {
        reject(new Error(`failed to parse barq-witness JSON output: ${String(e)}`));
        return;
      }

      // Normalise missing arrays so callers can iterate safely.
      if (!Array.isArray(report.segments)) {
        report.segments = [];
      }
      for (const seg of report.segments) {
        if (!Array.isArray(seg.reason_codes)) {
          seg.reason_codes = [];
        }
      }

      resolve(report);
    });
  });
}
