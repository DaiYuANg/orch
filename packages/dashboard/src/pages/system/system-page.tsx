import { useCustom } from "@refinedev/core";
import { Cpu, HardDrive, MemoryStick, Server } from "lucide-react";

import { ListView, ListViewHeader } from "@/components/refine-ui/views/list-view";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

type SystemInfo = {
  hostname: string;
  uptime: number;
  os: string;
  platform: string;
  load1: number;
  load5: number;
  load15: number;
  kernelVersion: string;
  kernelArch: string;
};

type CPUInfo = {
  model_name: string;
  cores: number;
  percent: number[];
};

type MemInfo = {
  total: number;
  used: number;
  free: number;
  used_percent: number;
};

type DiskInfo = {
  device: string;
  mountpoint: string;
  total: number;
  used: number;
  free: number;
  used_percent: number;
};

const toGiB = (value: number) => `${(value / 1024 / 1024 / 1024).toFixed(1)} GiB`;

export function SystemPage() {
  const infoQuery = useCustom<SystemInfo>({ url: "/system/info", method: "get" });
  const cpuQuery = useCustom<CPUInfo>({ url: "/system/cpu", method: "get" });
  const memQuery = useCustom<MemInfo>({ url: "/system/mem", method: "get" });
  const diskQuery = useCustom<DiskInfo[]>({ url: "/system/disk", method: "get" });

  const info = infoQuery.result.data;
  const cpu = cpuQuery.result.data;
  const mem = memQuery.result.data;
  const disk = diskQuery.result.data?.[0];

  return (
    <ListView>
      <ListViewHeader title="System Overview" canCreate={false} />
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <Server size={14} />
              Host
            </CardTitle>
            <CardDescription>{info?.hostname || "loading..."}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-1 text-xs text-muted-foreground">
            <div>OS: {info?.os || "-"}</div>
            <div>Platform: {info?.platform || "-"}</div>
            <div>Kernel: {info?.kernelVersion || "-"} ({info?.kernelArch || "-"})</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <Cpu size={14} />
              CPU
            </CardTitle>
            <CardDescription>{cpu?.model_name || "loading..."}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-1 text-xs text-muted-foreground">
            <div>Cores: {cpu?.cores ?? "-"}</div>
            <div>
              Avg usage:{" "}
              {cpu?.percent?.length
                ? `${(cpu.percent.reduce((sum, value) => sum + value, 0) / cpu.percent.length).toFixed(1)}%`
                : "-"}
            </div>
            <div>Load1: {info?.load1?.toFixed(2) ?? "-"}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <MemoryStick size={14} />
              Memory
            </CardTitle>
            <CardDescription>{mem ? toGiB(mem.total) : "loading..."}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-1 text-xs text-muted-foreground">
            <div>Used: {mem ? toGiB(mem.used) : "-"}</div>
            <div>Free: {mem ? toGiB(mem.free) : "-"}</div>
            <div>Used pct: {mem ? `${mem.used_percent.toFixed(1)}%` : "-"}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <HardDrive size={14} />
              Disk
            </CardTitle>
            <CardDescription>{disk?.mountpoint || "loading..."}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-1 text-xs text-muted-foreground">
            <div>Total: {disk ? toGiB(disk.total) : "-"}</div>
            <div>Used: {disk ? toGiB(disk.used) : "-"}</div>
            <div>Used pct: {disk ? `${disk.used_percent.toFixed(1)}%` : "-"}</div>
          </CardContent>
        </Card>
      </div>
    </ListView>
  );
}
