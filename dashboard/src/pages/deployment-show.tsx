import { useOne, useUpdate } from "@refinedev/core";
import { RotateCcw } from "lucide-react";
import { useMemo } from "react";
import { useParams } from "react-router";

import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";

type Deployment = {
  id: string;
  workload: string;
  status: "running" | "failed" | "stopped" | string;
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

export function DeploymentShowPage() {
  const { id } = useParams();
  const validID = id ?? "";
  const { result, query } = useOne<DeploymentDetail>({
    resource: "deployments",
    id: validID,
    queryOptions: {
      enabled: validID !== "",
    },
  });

  const stopMutation = useUpdate();
  const detail = result;
  const canStop = useMemo(() => detail?.deployment?.status === "running", [detail?.deployment?.status]);

  if (query.isLoading) {
    return (
      <Card>
        <CardContent className="py-8 text-sm text-[var(--muted)]">Loading deployment detail...</CardContent>
      </Card>
    );
  }

  if (!detail) {
    return (
      <Card>
        <CardContent className="py-8 text-sm text-[var(--muted)]">Deployment not found.</CardContent>
      </Card>
    );
  }

  return (
    <section className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="font-title text-2xl">{detail.deployment.workload}</h1>
          <p className="mt-1 text-sm text-[var(--muted)]">Deployment ID: {detail.deployment.id}</p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={statusVariant(detail.deployment.status)}>{detail.deployment.status}</Badge>
          <Button
            variant="secondary"
            size="sm"
            disabled={!canStop || stopMutation.mutation.isPending}
            onClick={() =>
              stopMutation.mutate({
                resource: "deployments",
                id: detail.deployment.id,
                values: {
                  action: "stop",
                },
              })
            }
          >
            <RotateCcw size={14} />
            Stop deployment
          </Button>
        </div>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Instances</CardTitle>
          <CardDescription>Runtime state of each instance inside this deployment.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3">
            {detail.instances.map((instance: Instance) => (
              <div
                key={instance.id}
                className="rounded-xl border border-[var(--line)] bg-[var(--panel-muted)]/70 p-4"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="text-sm font-semibold">{instance.service}</div>
                  <Badge variant={statusVariant(instance.status)}>{instance.status}</Badge>
                </div>
                <div className="mt-2 grid gap-1 text-xs text-[var(--muted)] md:grid-cols-2">
                  <div>ID: {instance.id}</div>
                  <div>Driver: {instance.driver}</div>
                  <div>Task: {instance.task}</div>
                  <div>Replica: {instance.replica}</div>
                  <div>Restart count: {instance.restart_count}</div>
                  <div>Last error: {instance.last_error || "-"}</div>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </section>
  );
}
