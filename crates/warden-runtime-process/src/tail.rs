use anyhow::Context;
use std::path::Path;

pub fn read_tail_lines(path: &Path, limit: usize) -> anyhow::Result<Vec<String>> {
  if !path.exists() {
    return Ok(Vec::new());
  }
  let content = std::fs::read_to_string(path)
    .with_context(|| format!("read process log file {}", path.display()))?;
  let all = content.lines().map(ToString::to_string).collect::<Vec<_>>();
  let start = all.len().saturating_sub(limit);
  Ok(all[start..].to_vec())
}
