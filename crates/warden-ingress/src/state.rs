use moka::sync::Cache;
use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::{Mutex, RwLock, oneshot, watch};
use warden_registry::RegistryService;

pub const DEFAULT_HTTP_CACHE_CAPACITY: u64 = 2048;
pub const DEFAULT_MAX_REQUEST_BODY: usize = 10 * 1024 * 1024;

#[derive(Debug, Clone)]
pub struct IngressOptions {
  pub http_cache_capacity: u64,
  pub http_cache_ttl: Duration,
  pub http_proxy_timeout: Duration,
  pub max_request_body: usize,
  pub stream_sync_interval: Duration,
  pub udp_backend_timeout: Duration,
}

impl Default for IngressOptions {
  fn default() -> Self {
    Self {
      http_cache_capacity: DEFAULT_HTTP_CACHE_CAPACITY,
      http_cache_ttl: Duration::from_secs(2),
      http_proxy_timeout: Duration::from_secs(10),
      max_request_body: DEFAULT_MAX_REQUEST_BODY,
      stream_sync_interval: Duration::from_secs(3),
      udp_backend_timeout: Duration::from_secs(3),
    }
  }
}

#[derive(Clone)]
pub struct IngressService {
  pub(crate) inner: Arc<IngressInner>,
}

pub(crate) struct IngressInner {
  pub(crate) listen_addr: String,
  pub(crate) registry: RegistryService,
  pub(crate) http_client: reqwest::Client,
  pub(crate) http_cache: Cache<String, String>,
  pub(crate) options: IngressOptions,
  pub(crate) tcp_listeners: Mutex<HashMap<u16, TcpListenerHandle>>,
  pub(crate) udp_listeners: Mutex<HashMap<u16, UdpListenerHandle>>,
  pub(crate) stop_tx: watch::Sender<bool>,
}

pub(crate) struct TcpListenerHandle {
  pub(crate) backend: Arc<RwLock<String>>,
  pub(crate) stop_tx: oneshot::Sender<()>,
}

pub(crate) struct UdpListenerHandle {
  pub(crate) backend: Arc<RwLock<String>>,
  pub(crate) stop_tx: oneshot::Sender<()>,
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
    let cache = Cache::builder()
      .max_capacity(options.http_cache_capacity)
      .time_to_live(options.http_cache_ttl)
      .build();

    Self {
      inner: Arc::new(IngressInner {
        listen_addr: addr,
        registry,
        http_client: reqwest::Client::builder()
          .timeout(options.http_proxy_timeout)
          .build()
          .unwrap_or_else(|_| reqwest::Client::new()),
        http_cache: cache,
        options,
        tcp_listeners: Mutex::new(HashMap::new()),
        udp_listeners: Mutex::new(HashMap::new()),
        stop_tx,
      }),
    }
  }
}
