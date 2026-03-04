import { useList } from "@refinedev/core";
import { Link } from "react-router";

import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { cn } from "../lib/utils";

type Deployment = {
  id: string;
  workload: string;
  status: "running" | "failed" | "stopped" | string;
  format: string;
  updated_at: string;
  instance_ids: string[];
  route_ids?: string[];
};

const statusVariant = (status: Deployment["status"]) => {
  if (status === "running") {
    return "success" as const;
  }
  if (status === "failed") {
    return "danger" as const;
  }
  if (status === "stopped") {
    return "warning" as const;
  }
  return "default" as const;
};

export function DeploymentsListPage() {
  const { result, query } = useList<Deployment>({
    resource: "deployments",
  });

  return (
    <section className="space-y-4">
      <header className="flex flex-col gap-1">
        <h1 className="font-title text-2xl">Deployments</h1>
        <p className="text-sm text-[var(--muted)]">
          Runtime tasks currently managed by this node.
        </p>
      </header>

      {query.isLoading ? (
        <Card>
          <CardContent className="py-8 text-sm text-[var(--muted)]">Loading deployments...</CardContent>
        </Card>
      ) : null}

      {!query.isLoading && result.data.length === 0 ? (
        <Card>
          <CardContent className="py-8 text-sm text-[var(--muted)]">No deployment found.</CardContent>
        </Card>
      ) : null}

      <div className="grid gap-4 md:grid-cols-2">
        {result.data.map((deployment) => (
          <Card key={deployment.id} className="hover:border-[var(--accent)] transition">
            <CardHeader>
              <div className="flex items-start justify-between gap-2">
                <div>
                  <CardTitle>{deployment.workload}</CardTitle>
                  <CardDescription className="mt-1">ID: {deployment.id}</CardDescription>
                </div>
                <Badge variant={statusVariant(deployment.status)}>{deployment.status}</Badge>
              </div>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="grid grid-cols-2 gap-3 text-sm">
                <div>
                  <div className="text-[var(--muted)]">Format</div>
                  <div className="mt-1 font-medium">{deployment.format}</div>
                </div>
                <div>
                  <div className="text-[var(--muted)]">Instances</div>
                  <div className="mt-1 font-medium">{deployment.instance_ids?.length ?? 0}</div>
                </div>
              </div>
              <Link
                to={`/deployments/${deployment.id}`}
                className={cn(
                  "inline-flex rounded-lg px-3 py-2 text-sm font-medium text-[var(--accent)]",
                  "hover:bg-[var(--panel-muted)]",
                )}
              >
                Open deployment detail
              </Link>
            </CardContent>
          </Card>
        ))}
      </div>
    </section>
  );
}
