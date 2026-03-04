import { Refine } from "@refinedev/core";
import routerProvider, {
  CatchAllNavigate,
  DocumentTitleHandler,
  NavigateToResource,
  UnsavedChangesNotifier,
} from "@refinedev/react-router";
import { BrowserRouter, Route, Routes } from "react-router";

import { DashboardLayout } from "./layout/dashboard-layout";
import { DeploymentShowPage } from "./pages/deployment-show";
import { DeploymentsListPage } from "./pages/deployments-list";
import { SystemOverviewPage } from "./pages/system-overview";
import { wardenDataProvider } from "./providers/wardenDataProvider";

function App() {
  return (
    <BrowserRouter>
      <Refine
        dataProvider={wardenDataProvider}
        routerProvider={routerProvider}
        options={{
          syncWithLocation: true,
          warnWhenUnsavedChanges: true,
        }}
        resources={[
          {
            name: "deployments",
            list: "/deployments",
            show: "/deployments/:id",
          },
          {
            name: "system",
            list: "/system",
          },
        ]}
      >
        <Routes>
          <Route path="/" element={<NavigateToResource resource="deployments" />} />
          <Route element={<DashboardLayout />}>
            <Route path="/deployments" element={<DeploymentsListPage />} />
            <Route path="/deployments/:id" element={<DeploymentShowPage />} />
            <Route path="/system" element={<SystemOverviewPage />} />
          </Route>
          <Route path="*" element={<CatchAllNavigate to="/deployments" />} />
        </Routes>
        <UnsavedChangesNotifier />
        <DocumentTitleHandler />
      </Refine>
    </BrowserRouter>
  );
}

export default App;
