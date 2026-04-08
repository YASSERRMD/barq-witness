export interface Segment {
  file_path: string;
  line_start: number;
  line_end: number;
  tier: number;
  score: number;
  reason_codes: string[];
  explanation?: string;
  prompt_text?: string;
  accepted_sec: number;
  executed: boolean;
}

export interface Report {
  commit_range: string;
  generated_at: number;
  total_segments: number;
  tier1_count: number;
  tier2_count: number;
  tier3_count: number;
  segments: Segment[];
}
