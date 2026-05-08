$ns = '{{ .Namespace }}'
$server = '{{ .Nameserver }}'
$existing = @(Get-DnsClientNrptRule -ErrorAction SilentlyContinue | Where-Object { $_.Namespace -eq $ns -or $_.Namespace -contains $ns })
$match = $existing | Where-Object { $_.NameServers -contains $server }
if ($match) { Write-Output 'installed' } else { Write-Output 'missing' }
