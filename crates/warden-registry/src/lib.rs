use warden_store::StateStore;
use warden_types::{EndpointRecord, RouteRecord, WorkloadSummary};

#[derive(Debug, Clone)]
pub struct RegistryService {
    store: StateStore,
}

impl RegistryService {
    pub fn new(store: StateStore) -> Self {
        Self { store }
    }

    pub async fn list_workloads(&self) -> Vec<WorkloadSummary> {
        self.store.list_workloads().await
    }

    pub async fn list_endpoints(&self) -> Vec<EndpointRecord> {
        self.store.list_endpoints().await
    }

    pub async fn list_routes(&self) -> Vec<RouteRecord> {
        self.store.list_routes().await
    }
}
