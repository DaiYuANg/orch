import { useCustom } from "@refinedev/core";
import { Activity, Cpu, HardDrive, Layers3, Server } from "lucide-react";

import { ListView, ListViewHeader } from "@/components/refine-ui/views/list-view";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

type ClusterNode = {
  node_id: string;
  workloads: number;
  endpoints: number;
  runtimes: string[];
  healthy: boolean;
};

type ClusterInfo = {
  raft_enabled: boolean;
  raft_node_id: number;
  raft_bind_addr: string;
  leader_node?: string;
  total_nodes: number;
  total_workloads: number;
  nodes: ClusterNode[];
};

type RuntimeManaged = {
  workload_id: string;
  runtime: string;
};

type RuntimeInfo = {
  providers: string[];
  managed: RuntimeManaged[];
};

type SystemInfo = {
  hostname: string;
  os: string;
  platform: string;
  kernel_version: string;
  kernel_arch: string;
  uptime: number;
};

const statusVariant = (healthy: boolean) => (healthy ? ("secondary" as const) : ("destructive" as const));

export function ClusterPage() {
  const clusterQuery = useCustom<ClusterInfo>({ url: "/system/cluster", method: "get" });
  const runtimeQuery = useCustom<RuntimeInfo>({ url: "/system/runtime", method: "get" });
  const systemQuery = useCustom<SystemInfo>({ url: "/system/info", method: "get" });

  const cluster = clusterQuery.result.data;
  const runtime = runtimeQuery.result.data;
  const system = systemQuery.result.data;

  return (
    <ListView>
      <ListViewHeader title="Cluster Overview" canCreate={false} />
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm"><Layers3 size={14} /> Nodes</CardTitle>
            <CardDescription>Total nodes in current view</CardDescription>
          </CardHeader>
          <CardContent className="text-2xl font-semibold">{cluster?.total_nodes ?? "-"}</CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm"><Activity size={14} /> Workloads</CardTitle>
            <CardDescription>Running + known workloads</CardDescription>
          </CardHeader>
          <CardContent className="text-2xl font-semibold">{cluster?.total_workloads ?? "-"}</CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm"><Server size={14} /> Leader</CardTitle>
            <CardDescription>Raft leader hint</CardDescription>
          </CardHeader>
          <CardContent className="space-y-1 text-sm">
            <div>{cluster?.leader_node || "n/a"}</div>
            <div className="text-xs text-muted-foreground">raft={cluster?.raft_enabled ? "enabled" : "disabled"}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm"><Cpu size={14} /> Host</CardTitle>
            <CardDescription>{system?.hostname || "loading..."}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-1 text-xs text-muted-foreground">
            <div>{system?.os || "-"} / {system?.platform || "-"}</div>
            <div>{system?.kernel_version || "-"} ({system?.kernel_arch || "-"})</div>
            <div>Uptime: {system ? `${Math.floor(system.uptime / 60)} min` : "-"}</div>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Cluster Nodes</CardTitle>
            <CardDescription>Node load and runtime distribution.</CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Node</TableHead>
                  <TableHead>Workloads</TableHead>
                  <TableHead>Endpoints</TableHead>
                  <TableHead>Runtimes</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(cluster?.nodes ?? []).map((node) => (
                  <TableRow key={node.node_id}>
                    <TableCell className="font-medium">{node.node_id}</TableCell>
                    <TableCell>{node.workloads}</TableCell>
                    <TableCell>{node.endpoints}</TableCell>
                    <TableCell>{node.runtimes.length > 0 ? node.runtimes.join(", ") : "-"}</TableCell>
                    <TableCell><Badge variant={statusVariant(node.healthy)}>{node.healthy ? "healthy" : "down"}</Badge></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2"><HardDrive size={14} /> Runtime Drivers</CardTitle>
            <CardDescription>Registered providers and managed workloads.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div>
              <div className="mb-1 text-xs text-muted-foreground">Providers</div>
              <div className="flex flex-wrap gap-2">
                {(runtime?.providers ?? []).map((name) => (
                  <Badge key={name} variant="outline">{name}</Badge>
                ))}
                {(runtime?.providers ?? []).length === 0 ? <span className="text-muted-foreground">No provider</span> : null}
              </div>
            </div>
            <div>
              <div className="mb-1 text-xs text-muted-foreground">Managed Workloads</div>
              <div className="max-h-52 overflow-auto rounded border p-2">
                {(runtime?.managed ?? []).map((row) => (
                  <div key={row.workload_id} className="flex items-center justify-between py-1 text-xs">
                    <span className="font-mono">{row.workload_id}</span>
                    <Badge variant="secondary">{row.runtime}</Badge>
                  </div>
                ))}
                {(runtime?.managed ?? []).length === 0 ? <div className="text-xs text-muted-foreground">No managed workload</div> : null}
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </ListView>
  );
}
