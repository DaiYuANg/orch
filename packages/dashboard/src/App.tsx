import { Refine } from "@refinedev/core";
import routerProvider, {
  CatchAllNavigate,
  DocumentTitleHandler,
  NavigateToResource,
  UnsavedChangesNotifier,
} from "@refinedev/react-router";
import { Activity, Globe2, Layers3, Network, Server } from "lucide-react";
import { BrowserRouter, Outlet, Route, Routes } from "react-router";

import { Layout } from "@/components/refine-ui/layout/layout";
import { useNotificationProvider } from "@/components/refine-ui/notification/use-notification-provider";
import { ClusterPage } from "@/pages/cluster/cluster-page";
import { DeploymentsListPage } from "@/pages/deployments/list-page";
import { DeploymentShowPage } from "@/pages/deployments/show-page";
import { DNSPage } from "@/pages/dns/dns-page";
import { NetworkPage } from "@/pages/network/network-page";
import { SystemPage } from "@/pages/system/system-page";
import { wardenDataProvider } from "@/providers/warden-data-provider";

function AppContent() {
  const notificationProvider = useNotificationProvider();

  return (
    <Refine
      dataProvider={wardenDataProvider}
      routerProvider={routerProvider}
      notificationProvider={notificationProvider}
      options={{
        syncWithLocation: true,
        warnWhenUnsavedChanges: true,
        disableTelemetry: true,
        title: {
          icon: <Server className="h-4 w-4" />,
          text: "Warden",
        },
      }}
      resources={[
        {
          name: "cluster",
          list: "/cluster",
          meta: {
            label: "Cluster",
            icon: <Activity className="h-4 w-4" />,
          },
        },
        {
          name: "deployments",
          list: "/deployments",
          show: "/deployments/:id",
          meta: {
            label: "Deployments",
            icon: <Layers3 className="h-4 w-4" />,
          },
        },
        {
          name: "system",
          list: "/system",
          meta: {
            label: "System",
            icon: <Server className="h-4 w-4" />,
          },
        },
        {
          name: "network",
          list: "/network",
          meta: {
            label: "Network",
            icon: <Network className="h-4 w-4" />,
          },
        },
        {
          name: "dns",
          list: "/dns",
          meta: {
            label: "DNS",
            icon: <Globe2 className="h-4 w-4" />,
          },
        },
      ]}
    >
      <Routes>
        <Route
          element={
            <Layout>
              <Outlet />
            </Layout>
          }
        >
          <Route index element={<NavigateToResource resource="cluster" />} />
          <Route path="/cluster" element={<ClusterPage />} />
          <Route path="/deployments" element={<DeploymentsListPage />} />
          <Route path="/deployments/:id" element={<DeploymentShowPage />} />
          <Route path="/system" element={<SystemPage />} />
          <Route path="/network" element={<NetworkPage />} />
          <Route path="/dns" element={<DNSPage />} />
        </Route>
        <Route path="*" element={<CatchAllNavigate to="/cluster" />} />
      </Routes>
      <UnsavedChangesNotifier />
      <DocumentTitleHandler />
    </Refine>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AppContent />
    </BrowserRouter>
  );
}
