use crate::cli::PackageArgs;
use crate::util::{copy_file, run_command};
use anyhow::Context;
use std::fs;
use std::path::Path;

pub fn package_release(root: &Path, args: &PackageArgs) -> anyhow::Result<()> {
  run_command(
    root,
    "cargo",
    &[
      "build",
      "--release",
      "-p",
      "warden-server",
      "-p",
      "warden-cli",
    ],
  )?;

  let out_dir = root.join(&args.out_dir);
  fs::create_dir_all(&out_dir).with_context(|| format!("create {}", out_dir.display()))?;

  let exe_suffix = if cfg!(windows) { ".exe" } else { "" };
  let server_bin = format!("warden-server{exe_suffix}");
  let cli_bin = format!("warden-cli{exe_suffix}");
  let target_dir = root.join("target").join("release");

  copy_file(
    &target_dir.join(&server_bin),
    &out_dir.join(&server_bin),
    "server binary",
  )?;
  copy_file(
    &target_dir.join(&cli_bin),
    &out_dir.join(&cli_bin),
    "cli binary",
  )?;

  println!("package output: {}", out_dir.display());
  Ok(())
}
