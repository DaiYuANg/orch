use warden_dsl::{build_plan, compile_manifest};

fn sample_manifest() -> warden_dsl::ApplicationManifest {
  serde_yaml::from_str(
    r#"
apiVersion: warden.io/v1alpha1
kind: Application
metadata:
  name: demo
  namespace: default
spec:
  workloads:
    - name: web
      runtime: docker
      image: nginx:stable-alpine
      service:
        port: 8080
      ingress:
        host: demo.local
        path: /
        listenPort: 18088
    - name: worker
      runtime: firecracker
      firecracker:
        config: ./fc.json
"#,
  )
  .expect("decode manifest")
}

#[test]
fn compile_generates_stable_workload_names() {
  let manifest = sample_manifest();
  let compiled = compile_manifest(&manifest).expect("compile manifest");

  assert_eq!(compiled.prefix, "default.demo.");
  assert_eq!(compiled.workloads[0].name, "default.demo.web");
  assert_eq!(compiled.workloads[1].name, "default.demo.worker");
}

#[test]
fn plan_detects_create_keep_delete() {
  let manifest = sample_manifest();
  let compiled = compile_manifest(&manifest).expect("compile manifest");
  let existing = vec![
    String::from("default.demo.web"),
    String::from("default.demo.old"),
    String::from("other.app.item"),
  ];
  let plan = build_plan(&compiled, &existing);

  assert_eq!(plan.create, vec![String::from("default.demo.worker")]);
  assert_eq!(plan.keep, vec![String::from("default.demo.web")]);
  assert_eq!(
    plan.delete_candidates,
    vec![String::from("default.demo.old")]
  );
}

#[test]
fn validation_rejects_wrong_api_version() {
  let bad: warden_dsl::ApplicationManifest = serde_yaml::from_str(
    r#"
apiVersion: warden.io/v9
kind: Application
metadata:
  name: demo
spec:
  workloads:
    - name: web
"#,
  )
  .expect("decode manifest");
  let err = bad.validate().expect_err("should fail");
  assert!(err.to_string().contains("apiVersion"));
}

#[test]
fn validation_requires_process_command_for_process_runtime() {
  let bad: warden_dsl::ApplicationManifest = serde_yaml::from_str(
    r#"
apiVersion: warden.io/v1alpha1
kind: Application
metadata:
  name: demo
spec:
  workloads:
    - name: worker
      runtime: process
"#,
  )
  .expect("decode manifest");
  let err = bad.validate().expect_err("should fail");
  assert!(err.to_string().contains("process"));
}

#[test]
fn compile_maps_process_runtime_fields() {
  let manifest: warden_dsl::ApplicationManifest = serde_yaml::from_str(
    r#"
apiVersion: warden.io/v1alpha1
kind: Application
metadata:
  name: demo
spec:
  workloads:
    - name: worker
      runtime: process
      process:
        command: sh
        args: ["-c", "sleep 1"]
        env:
          A: B
"#,
  )
  .expect("decode manifest");

  let compiled = compile_manifest(&manifest).expect("compile manifest");
  let req = &compiled.workloads[0].request;
  assert_eq!(req.runtime, "process");
  assert_eq!(req.process_command.as_deref(), Some("sh"));
  assert_eq!(
    req.process_args,
    vec![String::from("-c"), String::from("sleep 1")]
  );
  assert_eq!(req.process_env.get("A").map(String::as_str), Some("B"));
}
