$ns = '{{ .Namespace }}'
$existing = @(Get-DnsClientNrptRule -ErrorAction SilentlyContinue | Where-Object { $_.Namespace -eq $ns -or $_.Namespace -contains $ns })
foreach ($rule in $existing) { Remove-DnsClientNrptRule -Name $rule.Name -Force }
