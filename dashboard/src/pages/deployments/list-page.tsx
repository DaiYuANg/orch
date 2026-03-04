import { useList, useUpdate } from "@refinedev/core";

import { DeleteButton } from "@/components/refine-ui/buttons/delete";
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
import { Button } from "@/components/ui/button";

type Deployment = {
  id: string;
  workload: string;
  status: "running" | "failed" | "stopped" | string;
  format: string;
  updated_at: string;
  instance_ids: string[];
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
  const stopMutation = useUpdate();

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
                  <TableHead>Format</TableHead>
                  <TableHead>Instances</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {result.data.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{item.workload}</TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(item.status)}>{item.status}</Badge>
                    </TableCell>
                    <TableCell>{item.format}</TableCell>
                    <TableCell>{item.instance_ids?.length ?? 0}</TableCell>
                    <TableCell className="flex justify-end gap-2">
                      <ShowButton resource="deployments" recordItemId={item.id} size="sm" />
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={item.status !== "running" || stopMutation.mutation.isPending}
                        onClick={() =>
                          stopMutation.mutate({
                            resource: "deployments",
                            id: item.id,
                            values: { action: "stop" },
                          })
                        }
                      >
                        Stop
                      </Button>
                      <DeleteButton
                        resource="deployments"
                        recordItemId={item.id}
                        variant="outline"
                        size="sm"
                      >
                        Stop (Confirm)
                      </DeleteButton>
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
