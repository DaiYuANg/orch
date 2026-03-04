import { useCustom } from "@refinedev/core";

import { ListView, ListViewHeader } from "@/components/refine-ui/views/list-view";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

type RouteItem = {
  id: string;
  owner_id: string;
  service: string;
  protocol: string;
  host?: string;
  path_prefix?: string;
  listen_port?: number;
  target_port?: number;
  enabled: boolean;
  source?: string;
};

type EndpointItem = {
  id: string;
  service: string;
  node_id: string;
  node_ip: string;
  runtime: string;
  protocol: string;
  healthy: boolean;
  ports?: Record<string, number>;
};

const healthyVariant = (healthy: boolean) => (healthy ? ("secondary" as const) : ("destructive" as const));
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
            {routesQuery.query.isLoading ? (
              <div className="p-4 text-sm text-muted-foreground">Loading routes...</div>
            ) : null}
            {!routesQuery.query.isLoading && routes.length === 0 ? (
              <div className="p-4 text-sm text-muted-foreground">No routes found.</div>
            ) : null}
            {!routesQuery.query.isLoading && routes.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Service</TableHead>
                    <TableHead>Protocol</TableHead>
                    <TableHead>Match</TableHead>
                    <TableHead>Target</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {routes.map((item) => (
                    <TableRow key={item.id}>
                      <TableCell className="font-medium">{item.service}</TableCell>
                      <TableCell>{item.protocol}</TableCell>
                      <TableCell className="max-w-[260px] truncate">
                        {item.protocol === "http"
                          ? `${item.host || "*"}${item.path_prefix || "/"}`
                          : `:${item.listen_port ?? 0}`}
                      </TableCell>
                      <TableCell>{item.target_port ?? "-"}</TableCell>
                      <TableCell>
                        <Badge variant={routeVariant(item.enabled)}>
                          {item.enabled ? "enabled" : "disabled"}
                        </Badge>
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
            <CardDescription>Healthy backends discovered by registry.</CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            {endpointsQuery.query.isLoading ? (
              <div className="p-4 text-sm text-muted-foreground">Loading endpoints...</div>
            ) : null}
            {!endpointsQuery.query.isLoading && endpoints.length === 0 ? (
              <div className="p-4 text-sm text-muted-foreground">No endpoints found.</div>
            ) : null}
            {!endpointsQuery.query.isLoading && endpoints.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Service</TableHead>
                    <TableHead>Node</TableHead>
                    <TableHead>Runtime</TableHead>
                    <TableHead>Ports</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {endpoints.map((item) => (
                    <TableRow key={item.id}>
                      <TableCell className="font-medium">{item.service}</TableCell>
                      <TableCell className="max-w-[240px] truncate">
                        {item.node_id} ({item.node_ip})
                      </TableCell>
                      <TableCell>{item.runtime || "-"}</TableCell>
                      <TableCell className="max-w-[220px] truncate">
                        {item.ports
                          ? Object.entries(item.ports)
                              .map(([key, value]) => `${key}:${value}`)
                              .join(", ")
                          : "-"}
                      </TableCell>
                      <TableCell>
                        <Badge variant={healthyVariant(item.healthy)}>
                          {item.healthy ? "healthy" : "unhealthy"}
                        </Badge>
                      </TableCell>
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
