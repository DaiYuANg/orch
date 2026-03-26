use warden_dsl::{compile_manifest, parse_manifest_source};

#[test]
fn parses_invocation_style_services_and_let_expressions() {
  let raw = r#"
app("mall") {
    let env = "prod"
    let version = "1.2.3"

    services {
        val redis = create("redis") {
            runtime(container)
            image("redis:7")
            expose("redis") {
                container(6379)
            }
        }

        val postgres = create("postgres") {
            runtime(container)
            image("postgres:16")
        }

        val gateway = create("gateway") {
            runtime(container)
            image("ghcr.io/acme/gateway:${version}")
            replicas(if env == "prod" then 3 else 1)
            dependsOn(redis, postgres)
        }
    }

    ingress("gateway-public") {
        host("mall.example.com")
        route("/") {
            backend(services.gateway)
            port("http")
        }
    }
}
"#;

  let manifest = parse_manifest_source(raw).expect("parse invocation manifest");
  assert_eq!(manifest.metadata.name, "mall");
  assert_eq!(manifest.metadata.namespace, "default");
  assert_eq!(manifest.spec.workloads.len(), 3);

  let gateway = manifest
    .spec
    .workloads
    .iter()
    .find(|item| item.name == "gateway")
    .expect("gateway workload");
  assert_eq!(gateway.runtime, "docker");
  assert_eq!(gateway.image.as_deref(), Some("ghcr.io/acme/gateway:1.2.3"));
  assert_eq!(gateway.replicas, Some(3));
  assert_eq!(
    gateway.ingress.as_ref().and_then(|v| v.host.as_deref()),
    Some("mall.example.com")
  );
  assert_eq!(
    gateway.ingress.as_ref().and_then(|v| v.path.as_deref()),
    Some("/")
  );
  assert_eq!(
    gateway.depends_on,
    vec![String::from("redis"), String::from("postgres")]
  );
  let redis = manifest
    .spec
    .workloads
    .iter()
    .find(|item| item.name == "redis")
    .expect("redis workload");
  assert_eq!(redis.service.as_ref().and_then(|v| v.port), Some(6379));
}

#[test]
fn invocation_compile_emits_transition_warnings() {
  let raw = r#"
app("mall") {
    services {
        val redis = create("redis") {
            runtime(container)
            image("redis:7")
        }
        val gateway = create("gateway") {
            runtime(container)
            image("ghcr.io/acme/gateway:1.2.3")
            replicas(2)
            dependsOn(redis)
        }
    }
}
"#;

  let manifest = parse_manifest_source(raw).expect("parse invocation manifest");
  let compiled = compile_manifest(&manifest).expect("compile invocation manifest");
  assert!(
    compiled
      .warnings
      .iter()
      .any(|item| item.contains("replicas"))
  );
}

#[test]
fn parse_manifest_source_keeps_yaml_compatibility() {
  let yaml = r#"
apiVersion: warden.io/v1alpha1
kind: Application
metadata:
  name: demo
spec:
  workloads:
    - name: web
      runtime: docker
      image: nginx:stable-alpine
"#;

  let manifest = parse_manifest_source(yaml).expect("parse yaml manifest");
  assert_eq!(manifest.metadata.name, "demo");
  assert_eq!(manifest.spec.workloads.len(), 1);
}
