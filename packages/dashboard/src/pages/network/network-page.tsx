import { useCustom } from "@refinedev/core";

import { ListView, ListViewHeader } from "@/components/refine-ui/views/list-view";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

type RouteItem = {
  id: string;
  protocol: string;
  host: string;
  path_prefix: string;
  listen_port: number;
  backend: string;
  enabled: boolean;
};

type EndpointItem = {
  workload_id: string;
  node_id: string;
  protocol: string;
  address: string;
};

const routeVariant = (enabled: boolean) => (enabled ? ("secondary" as const) : ("outline" as const));

export function NetworkPage() {
  const routesQuery = useCustom<RouteItem[]>({ url: "/system/routes", method: "get" });
  const endpointsQuery = useCustom<EndpointItem[]>({ url: "/system/endpoints", method: "get" });
  const routes = routesQuery.result.data ?? [];
  const endpoints = endpointsQuery.result.data ?? [];

  return (
    <ListView>
      <ListViewHeader title="Network Overview" canCreate={false} />
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Routes</CardTitle>
            <CardDescription>Ingress routes from registry.</CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            {routesQuery.query.isLoading ? <div className="p-4 text-sm text-muted-foreground">Loading routes...</div> : null}
            {!routesQuery.query.isLoading && routes.length === 0 ? <div className="p-4 text-sm text-muted-foreground">No routes found.</div> : null}
            {!routesQuery.query.isLoading && routes.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>Protocol</TableHead>
                    <TableHead>Match</TableHead>
                    <TableHead>Backend</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {routes.map((item) => (
                    <TableRow key={item.id}>
                      <TableCell className="font-medium">{item.id}</TableCell>
                      <TableCell>{item.protocol}</TableCell>
                      <TableCell>{`${item.host}${item.path_prefix}`}</TableCell>
                      <TableCell>{item.backend}</TableCell>
                      <TableCell>
                        <Badge variant={routeVariant(item.enabled)}>{item.enabled ? "enabled" : "disabled"}</Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : null}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Endpoints</CardTitle>
            <CardDescription>Backends discovered by registry.</CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            {endpointsQuery.query.isLoading ? <div className="p-4 text-sm text-muted-foreground">Loading endpoints...</div> : null}
            {!endpointsQuery.query.isLoading && endpoints.length === 0 ? (
              <div className="p-4 text-sm text-muted-foreground">No endpoints found.</div>
            ) : null}
            {!endpointsQuery.query.isLoading && endpoints.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Workload</TableHead>
                    <TableHead>Node</TableHead>
                    <TableHead>Protocol</TableHead>
                    <TableHead>Address</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {endpoints.map((item) => (
                    <TableRow key={`${item.workload_id}-${item.node_id}-${item.protocol}`}>
                      <TableCell className="font-medium">{item.workload_id}</TableCell>
                      <TableCell>{item.node_id}</TableCell>
                      <TableCell>{item.protocol}</TableCell>
                      <TableCell>{item.address}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : null}
          </CardContent>
        </Card>
      </div>
    </ListView>
  );
}
