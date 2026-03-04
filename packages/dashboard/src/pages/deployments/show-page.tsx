import { useCustom, useOne } from "@refinedev/core";
import { useParams } from "react-router";

import { ShowView, ShowViewHeader } from "@/components/refine-ui/views/show-view";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

type Deployment = {
  id: string;
  name: string;
  status: string;
  runtime: string;
  node_id: string;
  created_at: string;
};

type Endpoint = {
  workload_id: string;
  node_id: string;
  protocol: string;
  address: string;
};

type RouteItem = {
  id: string;
  protocol: string;
  host: string;
  path_prefix: string;
  listen_port: number;
  backend: string;
  enabled: boolean;
};

type DNSRecord = {
  domain: string;
  values: string[];
  ttl: number;
};

const statusVariant = (status: string) => {
  if (status === "running") return "secondary" as const;
  if (status === "failed") return "destructive" as const;
  if (status === "stopped") return "outline" as const;
  return "default" as const;
};

export function DeploymentShowPage() {
  const { id } = useParams();
  const deployment = useOne<Deployment>({
    resource: "deployments",
    id: id ?? "",
    queryOptions: { enabled: Boolean(id) },
  });
  const endpointsQuery = useCustom<Endpoint[]>({ url: "/system/endpoints", method: "get" });
  const routesQuery = useCustom<RouteItem[]>({ url: "/system/routes", method: "get" });
  const dnsQuery = useCustom<DNSRecord[]>({ url: "/system/dns/records", method: "get" });

  if (deployment.query.isLoading) {
    return <div className="p-4 text-sm text-muted-foreground">Loading deployment...</div>;
  }
  if (!deployment.result) {
    return <div className="p-4 text-sm text-muted-foreground">Deployment not found.</div>;
  }

  const item = deployment.result;
  const endpoints = (endpointsQuery.result.data ?? []).filter((entry) => entry.workload_id === item.id);
  const routes = (routesQuery.result.data ?? []).filter((entry) => entry.id.includes(item.id));
  const routeHosts = new Set(routes.map((entry) => entry.host));
  const dns = (dnsQuery.result.data ?? []).filter((entry) => routeHosts.has(entry.domain));

  return (
    <ShowView>
      <ShowViewHeader title={item.name} showActions={false} />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between">
            <span>Workload Summary</span>
            <Badge variant={statusVariant(item.status)}>{item.status}</Badge>
          </CardTitle>
          <CardDescription>ID: {item.id}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-2 text-sm md:grid-cols-2">
          <div>Runtime: {item.runtime}</div>
          <div>Node: {item.node_id}</div>
          <div>Created: {item.created_at}</div>
          <div>Endpoints: {endpoints.length}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Network Bindings</CardTitle>
          <CardDescription>Route and endpoint mappings for this workload.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Type</TableHead>
                <TableHead>Protocol</TableHead>
                <TableHead>Address</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {routes.map((entry) => (
                <TableRow key={entry.id}>
                  <TableCell className="font-medium">route</TableCell>
                  <TableCell>{entry.protocol}</TableCell>
                  <TableCell>{`${entry.host}${entry.path_prefix} -> ${entry.backend}`}</TableCell>
                  <TableCell>{entry.enabled ? "enabled" : "disabled"}</TableCell>
                </TableRow>
              ))}
              {endpoints.map((entry) => (
                <TableRow key={`${entry.workload_id}-${entry.node_id}-${entry.protocol}`}>
                  <TableCell className="font-medium">endpoint</TableCell>
                  <TableCell>{entry.protocol}</TableCell>
                  <TableCell>{entry.address}</TableCell>
                  <TableCell>{entry.node_id}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>DNS Records</CardTitle>
          <CardDescription>Resolved DNS records linked by route hostnames.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          {dns.length === 0 ? <div className="text-muted-foreground">No DNS records found.</div> : null}
          {dns.map((entry) => (
            <div key={entry.domain}>
              {entry.domain}: {entry.values.join(", ")} (ttl={entry.ttl})
            </div>
          ))}
        </CardContent>
      </Card>
    </ShowView>
  );
}
