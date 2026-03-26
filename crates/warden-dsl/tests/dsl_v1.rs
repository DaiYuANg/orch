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
fn compile_emits_explicit_ingress_routes() {
  let manifest = sample_manifest();
  let compiled = compile_manifest(&manifest).expect("compile manifest");

  assert_eq!(compiled.ingress_routes.len(), 1);
  let route = &compiled.ingress_routes[0];
  assert_eq!(route.id, "route-default.demo.web");
  assert_eq!(route.protocol, "http");
  assert_eq!(route.host, "demo.local");
  assert_eq!(route.path_prefix, "/");
  assert_eq!(route.listen_port, 18088);
  assert_eq!(route.backend_workload_name, "default.demo.web");
  assert_eq!(route.backend_endpoint_name, "http");
  assert!(route.dns_enabled);
  assert_eq!(route.dns_ttl, 60);
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

#[test]
fn validation_rejects_invalid_namespace() {
  let bad: warden_dsl::ApplicationManifest = serde_yaml::from_str(
    r#"
apiVersion: warden.io/v1alpha1
kind: Application
metadata:
  name: demo
  namespace: "bad space"
spec:
  workloads:
    - name: web
"#,
  )
  .expect("decode manifest");
  let err = bad.validate().expect_err("should fail");
  assert!(err.to_string().contains("metadata.namespace"));
}

#[test]
fn validation_rejects_zero_ports() {
  let bad: warden_dsl::ApplicationManifest = serde_yaml::from_str(
    r#"
apiVersion: warden.io/v1alpha1
kind: Application
metadata:
  name: demo
spec:
  workloads:
    - name: web
      service:
        port: 0
"#,
  )
  .expect("decode manifest");
  let err = bad.validate().expect_err("should fail");
  assert!(err.to_string().contains("service.port"));
}

#[test]
fn compile_keeps_runtime_symbol_and_sorts_warnings() {
  let manifest: warden_dsl::ApplicationManifest = serde_yaml::from_str(
    r#"
apiVersion: warden.io/v1alpha1
kind: Application
metadata:
  name: demo
spec:
  workloads:
    - name: web
      runtime: docker
      dns:
        enabled: false
        ttl: 30
      ingress:
        enabled: false
      scheduling:
        stateful: true
"#,
  )
  .expect("decode manifest");

  let compiled = compile_manifest(&manifest).expect("compile manifest");
  assert_eq!(compiled.workloads[0].request.runtime, "docker");
  assert_eq!(compiled.warnings.len(), 1);
  assert!(compiled.warnings[0].contains("scheduling"));
}

#[test]
fn validation_rejects_non_symbol_runtime_value() {
  let bad: warden_dsl::ApplicationManifest = serde_yaml::from_str(
    r#"
apiVersion: warden.io/v1alpha1
kind: Application
metadata:
  name: demo
spec:
  workloads:
    - name: web
      runtime: DOCKER
"#,
  )
  .expect("decode manifest");

  let err = bad.validate().expect_err("should fail");
  assert!(err.to_string().contains("unsupported runtime"));
}

#[test]
fn validation_rejects_unknown_dependency() {
  let bad: warden_dsl::ApplicationManifest = serde_yaml::from_str(
    r#"
apiVersion: warden.io/v1alpha1
kind: Application
metadata:
  name: demo
spec:
  workloads:
    - name: web
      runtime: docker
      dependsOn: [db]
"#,
  )
  .expect("decode manifest");

  let err = bad.validate().expect_err("should fail");
  assert!(err.to_string().contains("depends on unknown workload"));
}
