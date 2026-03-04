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

type DNSRecord = {
  domain: string;
  type: string;
  value: string;
  ttlSeconds: number;
  updatedAt: string;
};

export function DNSPage() {
  const recordsQuery = useCustom<DNSRecord[]>({ url: "/system/dns/records", method: "get" });
  const records = recordsQuery.result.data ?? [];

  return (
    <ListView>
      <ListViewHeader title="DNS Records" canCreate={false} />
      <Card>
        <CardHeader>
          <CardTitle>Records</CardTitle>
          <CardDescription>Persisted DNS records in Warden DNS store.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {recordsQuery.query.isLoading ? (
            <div className="p-4 text-sm text-muted-foreground">Loading records...</div>
          ) : null}
          {!recordsQuery.query.isLoading && records.length === 0 ? (
            <div className="p-4 text-sm text-muted-foreground">No DNS records found.</div>
          ) : null}
          {!recordsQuery.query.isLoading && records.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Domain</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Value</TableHead>
                  <TableHead>TTL</TableHead>
                  <TableHead>Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {records.map((item) => (
                  <TableRow key={`${item.domain}-${item.type}-${item.value}`}>
                    <TableCell className="font-medium">{item.domain}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{item.type}</Badge>
                    </TableCell>
                    <TableCell>{item.value}</TableCell>
                    <TableCell>{item.ttlSeconds}s</TableCell>
                    <TableCell>{item.updatedAt || "-"}</TableCell>
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
