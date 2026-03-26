use std::sync::Arc;
use tokio::sync::watch;
use warden_ingress_http::HttpIngressState;
use warden_ingress_stream::StreamIngressRuntime;
use warden_ingress_types::IngressOptions;
use warden_registry::RegistryService;

#[derive(Clone)]
pub struct IngressService {
  pub(crate) inner: Arc<IngressInner>,
}

pub(crate) struct IngressInner {
  pub(crate) listen_addr: String,
  pub(crate) registry: RegistryService,
  pub(crate) http: HttpIngressState,
  pub(crate) stream: StreamIngressRuntime,
  pub(crate) options: IngressOptions,
  pub(crate) stop_tx: watch::Sender<bool>,
}

impl IngressService {
  pub fn new(listen_addr: String, registry: RegistryService) -> Self {
    Self::with_options(listen_addr, registry, IngressOptions::default())
  }

  pub fn with_options(
    listen_addr: String,
    registry: RegistryService,
    options: IngressOptions,
  ) -> Self {
    let addr = if listen_addr.trim().is_empty() {
      String::from(":8088")
    } else {
      listen_addr
    };
    let (stop_tx, _) = watch::channel(false);
    let http = HttpIngressState::new(&options);

    Self {
      inner: Arc::new(IngressInner {
        listen_addr: addr,
        registry,
        http,
        stream: StreamIngressRuntime::new(),
        options,
        stop_tx,
      }),
    }
  }
}
