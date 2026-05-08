$ns = '{{ .Namespace }}'
$server = '{{ .Nameserver }}'
$existing = @(Get-DnsClientNrptRule -ErrorAction SilentlyContinue | Where-Object { $_.Namespace -eq $ns -or $_.Namespace -contains $ns })
foreach ($rule in $existing) { Remove-DnsClientNrptRule -Name $rule.Name -Force }
Add-DnsClientNrptRule -Namespace $ns -NameServers $server -Comment 'Managed by orch'
