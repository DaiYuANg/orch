import { useOne } from "@refinedev/core";
import { useParams } from "react-router";

import { ShowView, ShowViewHeader } from "@/components/refine-ui/views/show-view";
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

type Deployment = {
  id: string;
  workload: string;
  status: string;
  format: string;
  created_at: string;
  updated_at: string;
};

type Instance = {
  id: string;
  service: string;
  task: string;
  replica: number;
  driver: string;
  status: string;
  restart_count: number;
  last_error?: string;
};

type DeploymentDetail = {
  deployment: Deployment;
  instances: Instance[];
};

const statusVariant = (status: string) => {
  if (status === "running") {
    return "secondary" as const;
  }
  if (status === "failed") {
    return "destructive" as const;
  }
  if (status === "stopped") {
    return "outline" as const;
  }
  return "default" as const;
};

export function DeploymentShowPage() {
  const { id } = useParams();
  const { query, result } = useOne<DeploymentDetail>({
    resource: "deployments",
    id: id ?? "",
    queryOptions: {
      enabled: Boolean(id),
    },
  });

  if (query.isLoading) {
    return <div className="p-4 text-sm text-muted-foreground">Loading deployment...</div>;
  }

  if (!result) {
    return <div className="p-4 text-sm text-muted-foreground">Deployment not found.</div>;
  }

  return (
    <ShowView>
      <ShowViewHeader title={result.deployment.workload} />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between">
            <span>Deployment Summary</span>
            <Badge variant={statusVariant(result.deployment.status)}>
              {result.deployment.status}
            </Badge>
          </CardTitle>
          <CardDescription>ID: {result.deployment.id}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-2 text-sm md:grid-cols-2">
          <div>Format: {result.deployment.format}</div>
          <div>Updated: {result.deployment.updated_at}</div>
          <div>Created: {result.deployment.created_at}</div>
          <div>Instances: {result.instances.length}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Instances</CardTitle>
          <CardDescription>Current runtime status per instance.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Service</TableHead>
                <TableHead>Task</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Driver</TableHead>
                <TableHead>Replica</TableHead>
                <TableHead>Restarts</TableHead>
                <TableHead>Last Error</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {result.instances.map((instance) => (
                <TableRow key={instance.id}>
                  <TableCell className="font-medium">{instance.service}</TableCell>
                  <TableCell>{instance.task}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(instance.status)}>{instance.status}</Badge>
                  </TableCell>
                  <TableCell>{instance.driver}</TableCell>
                  <TableCell>{instance.replica}</TableCell>
                  <TableCell>{instance.restart_count}</TableCell>
                  <TableCell className="max-w-[260px] truncate">
                    {instance.last_error || "-"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </ShowView>
  );
}
