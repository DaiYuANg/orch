//! HIR/IR → [`warden_dsl::ApplicationManifest`] parity with the legacy string invocation parser.

use warden_dsl::{application_manifest_from_hir, parse_manifest_source};
use warden_dsl_hir::lower as lower_hir;
use warden_dsl_parser::parse;

#[test]
fn hir_manifest_matches_legacy_invocation_parse() {
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

  let ast = parse(raw).expect("parse dsl");
  let hir = lower_hir(&ast).expect("lower hir");
  let from_pipeline = application_manifest_from_hir(&hir).expect("manifest from hir");
  let legacy = parse_manifest_source(raw).expect("legacy parse");

  assert_eq!(from_pipeline.metadata.name, legacy.metadata.name);
  assert_eq!(from_pipeline.metadata.namespace, legacy.metadata.namespace);
  assert_eq!(
    from_pipeline.spec.workloads.len(),
    legacy.spec.workloads.len()
  );

  for leg in &legacy.spec.workloads {
    let from_p = from_pipeline
      .spec
      .workloads
      .iter()
      .find(|w| w.name == leg.name)
      .unwrap_or_else(|| panic!("missing workload {}", leg.name));
    assert_eq!(from_p.runtime, leg.runtime, "runtime {}", leg.name);
    assert_eq!(from_p.image, leg.image, "image {}", leg.name);
    assert_eq!(from_p.replicas, leg.replicas, "replicas {}", leg.name);
    assert_eq!(from_p.depends_on, leg.depends_on, "depends_on {}", leg.name);
    assert_eq!(
      from_p.service.as_ref().and_then(|s| s.port),
      leg.service.as_ref().and_then(|s| s.port),
      "service.port {}",
      leg.name
    );
    assert_eq!(
      from_p.ingress.as_ref().and_then(|i| i.host.as_deref()),
      leg.ingress.as_ref().and_then(|i| i.host.as_deref()),
      "ingress.host {}",
      leg.name
    );
    assert_eq!(
      from_p.ingress.as_ref().and_then(|i| i.path.as_deref()),
      leg.ingress.as_ref().and_then(|i| i.path.as_deref()),
      "ingress.path {}",
      leg.name
    );
  }
}
