use anyhow::{Context, bail};
use duct::cmd;
use std::fs;
use std::path::{Path, PathBuf};

pub fn run_command(root: &Path, program: &str, args: &[&str]) -> anyhow::Result<()> {
  let _ = run_command_capture(root, program, args)?;
  Ok(())
}

pub fn run_command_capture(root: &Path, program: &str, args: &[&str]) -> anyhow::Result<String> {
  let output = cmd(program, args)
    .dir(root)
    .stderr_to_stdout()
    .unchecked()
    .run()
    .with_context(|| format!("run command: {} {}", program, args.join(" ")))?;

  if !output.status.success() {
    let log = String::from_utf8_lossy(&output.stdout);
    bail!(
      "command failed with status {}: {} {}\n{}",
      output.status,
      program,
      args.join(" "),
      log
    );
  }

  Ok(String::from_utf8_lossy(&output.stdout).to_string())
}

pub fn copy_file(src: &Path, dst: &Path, name: &str) -> anyhow::Result<()> {
  if !src.exists() {
    bail!("{} missing: {}", name, src.display());
  }
  fs::copy(src, dst).with_context(|| format!("copy {} -> {}", src.display(), dst.display()))?;
  Ok(())
}

pub fn normalize_for_json(path: &Path) -> String {
  path.to_string_lossy().replace('\\', "/")
}

pub fn workspace_root() -> anyhow::Result<PathBuf> {
  let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
  manifest_dir
    .parent()
    .map(Path::to_path_buf)
    .context("resolve workspace root from xtask manifest dir")
}
