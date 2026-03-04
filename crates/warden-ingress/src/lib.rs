use axum::Router;
use axum::body::{Body, to_bytes};
use axum::extract::State;
use axum::http::header::HOST;
use axum::http::{Request, Response, StatusCode};
use axum::routing::any;
use std::collections::{HashMap, HashSet};
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::io::copy_bidirectional;
use tokio::net::{TcpListener, TcpStream, UdpSocket};
use tokio::sync::{Mutex, RwLock, oneshot, watch};
use tokio::time::timeout;
use tracing::{error, info, warn};
use warden_registry::RegistryService;
use warden_types::RouteRecord;

const HTTP_CACHE_CAPACITY: usize = 2048;
const HTTP_CACHE_TTL: Duration = Duration::from_secs(2);
const HTTP_PROXY_TIMEOUT: Duration = Duration::from_secs(10);
const MAX_REQUEST_BODY: usize = 10 * 1024 * 1024;
const STREAM_SYNC_INTERVAL: Duration = Duration::from_secs(3);

#[derive(Clone)]
pub struct IngressService {
    inner: Arc<IngressInner>,
}

struct IngressInner {
    listen_addr: String,
    registry: RegistryService,
    http_client: reqwest::Client,
    http_cache: Mutex<HashMap<String, HttpBackendCacheItem>>,
    tcp_listeners: Mutex<HashMap<u16, TcpListenerHandle>>,
    udp_listeners: Mutex<HashMap<u16, UdpListenerHandle>>,
    stop_tx: watch::Sender<bool>,
}

struct TcpListenerHandle {
    backend: Arc<RwLock<String>>,
    stop_tx: oneshot::Sender<()>,
}

struct UdpListenerHandle {
    backend: Arc<RwLock<String>>,
    stop_tx: oneshot::Sender<()>,
}

#[derive(Debug, Clone)]
struct HttpBackendCacheItem {
    backend: String,
    expires_at: Instant,
}

impl IngressService {
    pub fn new(listen_addr: String, registry: RegistryService) -> Self {
        let addr = if listen_addr.trim().is_empty() {
            String::from(":8088")
        } else {
            listen_addr
        };
        let (stop_tx, _) = watch::channel(false);
        Self {
            inner: Arc::new(IngressInner {
                listen_addr: addr,
                registry,
                http_client: reqwest::Client::builder()
                    .timeout(HTTP_PROXY_TIMEOUT)
                    .build()
                    .unwrap_or_else(|_| reqwest::Client::new()),
                http_cache: Mutex::new(HashMap::new()),
                tcp_listeners: Mutex::new(HashMap::new()),
                udp_listeners: Mutex::new(HashMap::new()),
                stop_tx,
            }),
        }
    }

    pub async fn start(&self) {
        info!(
            target: "warden::ingress",
            http_addr = %self.inner.listen_addr,
            "ingress startup begin"
        );
        self.start_http_listener().await;
        self.start_stream_sync_loop();
        info!(
            target: "warden::ingress",
            http_addr = %self.inner.listen_addr,
            "ingress startup complete"
        );
    }

    pub async fn stop(&self) {
        info!(target: "warden::ingress", "ingress shutdown begin");
        let _ = self.inner.stop_tx.send(true);
        self.shutdown_tcp_listeners().await;
        self.shutdown_udp_listeners().await;
        self.inner.http_cache.lock().await.clear();
        info!(target: "warden::ingress", "ingress shutdown complete");
    }

    async fn start_http_listener(&self) {
        let bind_addr = normalize_bind_addr(&self.inner.listen_addr);
        let listener = match TcpListener::bind(&bind_addr).await {
            Ok(item) => item,
            Err(err) => {
                error!(
                    target: "warden::ingress",
                    addr = %bind_addr,
                    error = %err,
                    "ingress http listen failed"
                );
                return;
            }
        };

        let mut stop_rx = self.inner.stop_tx.subscribe();
        let app = Router::new()
            .fallback(any(proxy_http_handler))
            .with_state(self.inner.clone());

        tokio::spawn(async move {
            let serve =
                axum::serve(listener, app.into_make_service()).with_graceful_shutdown(async move {
                    wait_for_stop(&mut stop_rx).await;
                });
            if let Err(err) = serve.await {
                error!(
                    target: "warden::ingress",
                    error = %err,
                    "ingress http listener stopped with error"
                );
                return;
            }
            info!(target: "warden::ingress", "ingress http listener stopped");
        });
    }

    fn start_stream_sync_loop(&self) {
        let inner = self.inner.clone();
        let mut stop_rx = inner.stop_tx.subscribe();
        tokio::spawn(async move {
            let mut ticker = tokio::time::interval(STREAM_SYNC_INTERVAL);
            loop {
                tokio::select! {
                    _ = ticker.tick() => {
                        sync_stream_routes(&inner, "tcp").await;
                        sync_stream_routes(&inner, "udp").await;
                    }
                    _ = stop_rx.changed() => {
                        if *stop_rx.borrow() {
                            return;
                        }
                    }
                }
            }
        });
    }

    async fn shutdown_tcp_listeners(&self) {
        let mut listeners = self.inner.tcp_listeners.lock().await;
        for (_, handle) in listeners.drain() {
            let _ = handle.stop_tx.send(());
        }
    }

    async fn shutdown_udp_listeners(&self) {
        let mut listeners = self.inner.udp_listeners.lock().await;
        for (_, handle) in listeners.drain() {
            let _ = handle.stop_tx.send(());
        }
    }
}

async fn proxy_http_handler(
    State(inner): State<Arc<IngressInner>>,
    req: Request<Body>,
) -> Response<Body> {
    let host = req
        .headers()
        .get(HOST)
        .and_then(|v| v.to_str().ok())
        .unwrap_or_default();
    let normalized_host = normalize_host_with_port(host);
    let path = req.uri().path().to_string();

    if let Some(backend) = get_cached_http_backend(&inner, &normalized_host, &path).await {
        return reverse_proxy_http(&inner, req, backend).await;
    }

    let backend = match resolve_http_backend(&inner, &normalized_host, &path).await {
        Some(item) => item,
        None => {
            return response_with_status(StatusCode::NOT_FOUND, "route not found");
        }
    };
    store_http_backend(&inner, &normalized_host, &path, &backend).await;
    reverse_proxy_http(&inner, req, backend).await
}

async fn reverse_proxy_http(
    inner: &Arc<IngressInner>,
    req: Request<Body>,
    backend: String,
) -> Response<Body> {
    let target = if let Some(value) = backend.strip_prefix("http://") {
        value.to_string()
    } else if backend.starts_with("https://") {
        return response_with_status(
            StatusCode::BAD_GATEWAY,
            "https backend is not supported yet",
        );
    } else {
        backend
    };

    let path_and_query = req
        .uri()
        .path_and_query()
        .map(|v| v.as_str().to_string())
        .unwrap_or_else(|| String::from("/"));
    let target_url = format!("http://{target}{path_and_query}");
    let (parts, body) = req.into_parts();
    let body_bytes = match to_bytes(body, MAX_REQUEST_BODY).await {
        Ok(payload) => payload,
        Err(err) => {
            return response_with_status(
                StatusCode::BAD_GATEWAY,
                &format!("proxy read body error: {err}"),
            );
        }
    };

    let mut outbound = inner
        .http_client
        .request(parts.method.clone(), target_url)
        .body(body_bytes.to_vec());

    for (name, value) in &parts.headers {
        if *name == HOST {
            continue;
        }
        outbound = outbound.header(name, value);
    }

    let upstream = match outbound.send().await {
        Ok(resp) => resp,
        Err(err) => {
            return response_with_status(StatusCode::BAD_GATEWAY, &format!("proxy error: {err}"));
        }
    };

    let status = upstream.status();
    let headers = upstream.headers().clone();
    let payload = match upstream.bytes().await {
        Ok(data) => data,
        Err(err) => {
            return response_with_status(StatusCode::BAD_GATEWAY, &format!("proxy error: {err}"));
        }
    };

    let mut response = Response::new(Body::from(payload));
    *response.status_mut() = status;
    for (name, value) in &headers {
        response.headers_mut().insert(name, value.clone());
    }
    response
}

async fn resolve_http_backend(inner: &Arc<IngressInner>, host: &str, path: &str) -> Option<String> {
    let routes = inner.registry.list_routes().await;
    let normalized_host = normalize_host_with_port(host);
    let normalized_path = ensure_path(path);

    let mut selected_backend: Option<String> = None;
    let mut longest = 0usize;

    for route in routes {
        if !route.enabled {
            continue;
        }
        if !route_protocol_match(&route, "http") {
            continue;
        }
        if !host_match(&route.host, &normalized_host) {
            continue;
        }
        let prefix = ensure_path(&route.path_prefix);
        if !normalized_path.starts_with(&prefix) {
            continue;
        }
        if prefix.len() <= longest {
            continue;
        }
        if route.backend.trim().is_empty() {
            continue;
        }
        longest = prefix.len();
        selected_backend = Some(route.backend.clone());
    }

    selected_backend
}

async fn sync_stream_routes(inner: &Arc<IngressInner>, protocol: &str) {
    let routes = inner.registry.list_routes().await;
    let mut active_ports = HashSet::new();

    for route in routes {
        if !route.enabled || route.listen_port == 0 || route.backend.trim().is_empty() {
            continue;
        }
        if !route_protocol_match(&route, protocol) {
            continue;
        }
        active_ports.insert(route.listen_port);
        if protocol == "tcp" {
            if let Err(err) = register_tcp(inner, route.listen_port, route.backend.clone()).await {
                warn!(
                    target: "warden::ingress",
                    protocol = "tcp",
                    listen_port = route.listen_port,
                    error = %err,
                    "register stream route failed"
                );
            }
        } else if let Err(err) = register_udp(inner, route.listen_port, route.backend.clone()).await
        {
            warn!(
                target: "warden::ingress",
                protocol = "udp",
                listen_port = route.listen_port,
                error = %err,
                "register stream route failed"
            );
        }
    }

    if protocol == "tcp" {
        unregister_inactive_tcp(inner, &active_ports).await;
    } else {
        unregister_inactive_udp(inner, &active_ports).await;
    }
}

async fn register_tcp(
    inner: &Arc<IngressInner>,
    listen_port: u16,
    backend: String,
) -> anyhow::Result<()> {
    let mut listeners = inner.tcp_listeners.lock().await;
    if let Some(existing) = listeners.get(&listen_port) {
        *existing.backend.write().await = backend;
        return Ok(());
    }

    let bind_addr = format!("0.0.0.0:{listen_port}");
    let listener = TcpListener::bind(&bind_addr).await?;
    let (stop_tx, stop_rx) = oneshot::channel();
    let backend_ref = Arc::new(RwLock::new(backend));
    listeners.insert(
        listen_port,
        TcpListenerHandle {
            backend: backend_ref.clone(),
            stop_tx,
        },
    );
    tokio::spawn(run_tcp_listener(listener, backend_ref, stop_rx));
    Ok(())
}

async fn run_tcp_listener(
    listener: TcpListener,
    backend: Arc<RwLock<String>>,
    mut stop_rx: oneshot::Receiver<()>,
) {
    loop {
        tokio::select! {
            _ = &mut stop_rx => {
                return;
            }
            accepted = listener.accept() => {
                match accepted {
                    Ok((stream, _)) => {
                        let backend_ref = backend.clone();
                        tokio::spawn(async move {
                            handle_tcp_connection(stream, backend_ref).await;
                        });
                    }
                    Err(err) => {
                        warn!(target: "warden::ingress", error = %err, "tcp accept error");
                    }
                }
            }
        }
    }
}

async fn handle_tcp_connection(mut client: TcpStream, backend: Arc<RwLock<String>>) {
    let backend_addr = backend.read().await.clone();
    let mut upstream = match TcpStream::connect(&backend_addr).await {
        Ok(item) => item,
        Err(err) => {
            warn!(
                target: "warden::ingress",
                backend = %backend_addr,
                error = %err,
                "tcp dial backend error"
            );
            return;
        }
    };

    if let Err(err) = copy_bidirectional(&mut client, &mut upstream).await {
        warn!(target: "warden::ingress", error = %err, "tcp proxy copy error");
    }
}

async fn unregister_inactive_tcp(inner: &Arc<IngressInner>, active_ports: &HashSet<u16>) {
    let mut listeners = inner.tcp_listeners.lock().await;
    let stale = listeners
        .keys()
        .filter(|port| !active_ports.contains(port))
        .copied()
        .collect::<Vec<_>>();
    for port in stale {
        if let Some(handle) = listeners.remove(&port) {
            let _ = handle.stop_tx.send(());
        }
    }
}

async fn register_udp(
    inner: &Arc<IngressInner>,
    listen_port: u16,
    backend: String,
) -> anyhow::Result<()> {
    let mut listeners = inner.udp_listeners.lock().await;
    if let Some(existing) = listeners.get(&listen_port) {
        *existing.backend.write().await = backend;
        return Ok(());
    }

    let bind_addr = format!("0.0.0.0:{listen_port}");
    let socket = Arc::new(UdpSocket::bind(&bind_addr).await?);
    let backend_ref = Arc::new(RwLock::new(backend));
    let (stop_tx, stop_rx) = oneshot::channel();
    listeners.insert(
        listen_port,
        UdpListenerHandle {
            backend: backend_ref.clone(),
            stop_tx,
        },
    );
    tokio::spawn(run_udp_listener(socket, backend_ref, stop_rx));
    Ok(())
}

async fn run_udp_listener(
    socket: Arc<UdpSocket>,
    backend: Arc<RwLock<String>>,
    mut stop_rx: oneshot::Receiver<()>,
) {
    let mut buf = vec![0u8; 64 * 1024];
    loop {
        tokio::select! {
            _ = &mut stop_rx => {
                return;
            }
            packet = socket.recv_from(&mut buf) => {
                match packet {
                    Ok((n, client_addr)) => {
                        let payload = buf[..n].to_vec();
                        let backend_ref = backend.clone();
                        let listener_socket = socket.clone();
                        tokio::spawn(async move {
                            handle_udp_packet(listener_socket, backend_ref, client_addr, payload).await;
                        });
                    }
                    Err(err) => {
                        warn!(target: "warden::ingress", error = %err, "udp read error");
                    }
                }
            }
        }
    }
}

async fn handle_udp_packet(
    listener_socket: Arc<UdpSocket>,
    backend: Arc<RwLock<String>>,
    client_addr: std::net::SocketAddr,
    payload: Vec<u8>,
) {
    let backend_addr = backend.read().await.clone();
    let upstream = match UdpSocket::bind("0.0.0.0:0").await {
        Ok(item) => item,
        Err(err) => {
            warn!(target: "warden::ingress", error = %err, "udp local bind error");
            return;
        }
    };

    if let Err(err) = upstream.send_to(&payload, &backend_addr).await {
        warn!(
            target: "warden::ingress",
            backend = %backend_addr,
            error = %err,
            "udp write to backend error"
        );
        return;
    }

    let mut resp = vec![0u8; 64 * 1024];
    let read = timeout(Duration::from_secs(3), upstream.recv_from(&mut resp)).await;
    let n = match read {
        Ok(Ok((n, _))) => n,
        Ok(Err(err)) => {
            warn!(
                target: "warden::ingress",
                backend = %backend_addr,
                error = %err,
                "udp read from backend error"
            );
            return;
        }
        Err(_) => {
            warn!(
                target: "warden::ingress",
                backend = %backend_addr,
                "udp read from backend timeout"
            );
            return;
        }
    };

    if let Err(err) = listener_socket.send_to(&resp[..n], client_addr).await {
        warn!(target: "warden::ingress", error = %err, "udp write to client error");
    }
}

async fn unregister_inactive_udp(inner: &Arc<IngressInner>, active_ports: &HashSet<u16>) {
    let mut listeners = inner.udp_listeners.lock().await;
    let stale = listeners
        .keys()
        .filter(|port| !active_ports.contains(port))
        .copied()
        .collect::<Vec<_>>();
    for port in stale {
        if let Some(handle) = listeners.remove(&port) {
            let _ = handle.stop_tx.send(());
        }
    }
}

async fn get_cached_http_backend(
    inner: &Arc<IngressInner>,
    host: &str,
    path: &str,
) -> Option<String> {
    let key = http_cache_key(host, path);
    let mut cache = inner.http_cache.lock().await;
    let item = cache.get(&key).cloned();
    match item {
        Some(entry) if Instant::now() <= entry.expires_at => Some(entry.backend),
        Some(_) => {
            cache.remove(&key);
            None
        }
        None => None,
    }
}

async fn store_http_backend(inner: &Arc<IngressInner>, host: &str, path: &str, backend: &str) {
    if backend.trim().is_empty() {
        return;
    }
    let key = http_cache_key(host, path);
    let mut cache = inner.http_cache.lock().await;
    if cache.len() >= HTTP_CACHE_CAPACITY {
        let now = Instant::now();
        cache.retain(|_, value| value.expires_at > now);
    }
    if cache.len() >= HTTP_CACHE_CAPACITY {
        if let Some(victim) = cache.keys().next().cloned() {
            cache.remove(&victim);
        }
    }
    cache.insert(
        key,
        HttpBackendCacheItem {
            backend: backend.to_string(),
            expires_at: Instant::now() + HTTP_CACHE_TTL,
        },
    );
}

fn route_protocol_match(route: &RouteRecord, protocol: &str) -> bool {
    route.protocol.trim().eq_ignore_ascii_case(protocol)
}

fn host_match(pattern: &str, host: &str) -> bool {
    let left = normalize_host_with_port(pattern);
    let right = normalize_host_with_port(host);
    left.is_empty() || left == "*" || left == right
}

fn ensure_path(path: &str) -> String {
    let value = path.trim();
    if value.is_empty() {
        return String::from("/");
    }
    if value.starts_with('/') {
        return value.to_string();
    }
    format!("/{value}")
}

fn normalize_host_with_port(host: &str) -> String {
    let raw = host.trim().to_ascii_lowercase();
    if raw.is_empty() {
        return raw;
    }
    if let Ok((host_only, _)) = split_host_port(&raw) {
        return host_only;
    }
    if raw.starts_with('[') && raw.ends_with(']') {
        return raw
            .trim_start_matches('[')
            .trim_end_matches(']')
            .to_string();
    }
    if let Some((host_only, _)) = raw.split_once(':') {
        return host_only.to_string();
    }
    raw
}

fn split_host_port(raw: &str) -> anyhow::Result<(String, u16)> {
    let addr: std::net::SocketAddr = raw.parse()?;
    Ok((addr.ip().to_string(), addr.port()))
}

fn normalize_bind_addr(addr: &str) -> String {
    let value = addr.trim();
    if let Some(port) = value.strip_prefix(':') {
        return format!("0.0.0.0:{port}");
    }
    value.to_string()
}

fn response_with_status(status: StatusCode, body: &str) -> Response<Body> {
    let mut response = Response::new(Body::from(body.to_string()));
    *response.status_mut() = status;
    response
}

fn http_cache_key(host: &str, path: &str) -> String {
    format!("{}|{}", host.trim().to_ascii_lowercase(), path.trim())
}

async fn wait_for_stop(stop_rx: &mut watch::Receiver<bool>) {
    if *stop_rx.borrow() {
        return;
    }
    while stop_rx.changed().await.is_ok() {
        if *stop_rx.borrow() {
            return;
        }
    }
}
