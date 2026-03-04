import { useList } from "@refinedev/core";

import { ShowButton } from "@/components/refine-ui/buttons/show";
import { ListView, ListViewHeader } from "@/components/refine-ui/views/list-view";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
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
  name: string;
  status: "running" | "failed" | "stopped" | string;
  runtime: string;
  node_id: string;
  created_at: string;
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

export function DeploymentsListPage() {
  const { query, result } = useList<Deployment>({ resource: "deployments" });

  return (
    <ListView>
      <ListViewHeader canCreate={false} />
      <Card>
        <CardContent className="p-0">
          {query.isLoading ? (
            <div className="p-6 text-sm text-muted-foreground">Loading deployments...</div>
          ) : null}
          {!query.isLoading && result.data.length === 0 ? (
            <div className="p-6 text-sm text-muted-foreground">No deployments found.</div>
          ) : null}
          {!query.isLoading && result.data.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Workload</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Runtime</TableHead>
                  <TableHead>Node</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {result.data.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{item.name}</TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(item.status)}>{item.status}</Badge>
                    </TableCell>
                    <TableCell>{item.runtime}</TableCell>
                    <TableCell>{item.node_id}</TableCell>
                    <TableCell>{item.created_at}</TableCell>
                    <TableCell className="text-right">
                      <ShowButton resource="deployments" recordItemId={item.id} size="sm" />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : null}
        </CardContent>
      </Card>
    </ListView>
  );
}
