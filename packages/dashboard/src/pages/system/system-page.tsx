import { useCustom } from "@refinedev/core";
import { Cpu, HardDrive, MemoryStick, Server } from "lucide-react";

import { ListView, ListViewHeader } from "@/components/refine-ui/views/list-view";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

type SystemInfo = {
  hostname: string;
  uptime: number;
  os: string;
  platform: string;
  kernel_version: string;
  kernel_arch: string;
};

type CPUInfo = {
  model_name: string;
  cores: number;
  usage_percent: number;
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

type MaybeWrappedArray<T> = T[] | { data?: unknown } | null | undefined;

const asNumber = (value: unknown): number | null => {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
};

const fmt1 = new Intl.NumberFormat(undefined, {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
});

const toGiB = (value: unknown) => {
  const numeric = asNumber(value);
  if (numeric === null) {
    return "-";
  }
  return `${fmt1.format(numeric / 1024 / 1024 / 1024)} GiB`;
};

const toPercent = (value: unknown) => {
  const numeric = asNumber(value);
  if (numeric === null) {
    return "-";
  }
  return `${fmt1.format(numeric)}%`;
};

const asArray = <T,>(value: MaybeWrappedArray<T>): T[] => {
  if (Array.isArray(value)) {
    return value;
  }
  if (value && typeof value === "object" && "data" in value && Array.isArray(value.data)) {
    return value.data as T[];
  }
  return [];
};

export function SystemPage() {
  const infoQuery = useCustom<SystemInfo>({ url: "/system/info", method: "get" });
  const cpuQuery = useCustom<CPUInfo>({ url: "/system/cpu", method: "get" });
  const memQuery = useCustom<MemInfo>({ url: "/system/mem", method: "get" });
  const diskQuery = useCustom<DiskInfo[]>({ url: "/system/disk", method: "get" });

  const info = infoQuery.result.data;
  const cpu = cpuQuery.result.data;
  const mem = memQuery.result.data;
  const disks = asArray<DiskInfo>(diskQuery.result.data as MaybeWrappedArray<DiskInfo>);
  const diskTotalBytes = disks.reduce((sum, item) => sum + (asNumber(item.total) ?? 0), 0);
  const diskUsedBytes = disks.reduce((sum, item) => sum + (asNumber(item.used) ?? 0), 0);
  const diskUsedPercent = diskTotalBytes === 0 ? null : (diskUsedBytes / diskTotalBytes) * 100;

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
            <div>Kernel: {info?.kernel_version || "-"} ({info?.kernel_arch || "-"})</div>
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
            <div>Usage: {toPercent(cpu?.usage_percent)}</div>
            <div>Uptime: {info ? `${Math.floor(info.uptime / 60)} min` : "-"}</div>
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
            <div>Used: {toGiB(mem?.used)}</div>
            <div>Free: {toGiB(mem?.free)}</div>
            <div>Used pct: {toPercent(mem?.used_percent)}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <HardDrive size={14} />
              Disks
            </CardTitle>
            <CardDescription>{disks.length > 0 ? `${disks.length} device(s)` : "loading..."}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-1 text-xs text-muted-foreground">
            <div>Total: {toGiB(diskTotalBytes)}</div>
            <div>Used: {toGiB(diskUsedBytes)}</div>
            <div>Used pct: {toPercent(diskUsedPercent)}</div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Disk Devices</CardTitle>
          <CardDescription>All detected host disks and mountpoints.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {diskQuery.query.isLoading ? (
            <div className="p-4 text-sm text-muted-foreground">Loading disks...</div>
          ) : null}
          {!diskQuery.query.isLoading && disks.length === 0 ? (
            <div className="p-4 text-sm text-muted-foreground">No disk data.</div>
          ) : null}
          {!diskQuery.query.isLoading && disks.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Device</TableHead>
                  <TableHead>Mount</TableHead>
                  <TableHead>Total</TableHead>
                  <TableHead>Used</TableHead>
                  <TableHead>Free</TableHead>
                  <TableHead>Used pct</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {disks.map((item) => (
                  <TableRow key={`${item.device}-${item.mountpoint}`}>
                    <TableCell className="font-medium">{item.device || "-"}</TableCell>
                    <TableCell>{item.mountpoint || "-"}</TableCell>
                    <TableCell>{toGiB(item.total)}</TableCell>
                    <TableCell>{toGiB(item.used)}</TableCell>
                    <TableCell>{toGiB(item.free)}</TableCell>
                    <TableCell>{toPercent(item.used_percent)}</TableCell>
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
