# Deploy

## Simple
```text
workload{
  "postgresql": {
    docker{
      image: "postgres:17"
      port: 5432
    }
  }
}
```


## Advance
```text
// postgresql.svcmod
module "PostgreSQL" (
  // 暴露给用户的参数 (自动生成配置表单)
  param version: string = "15" 
    options ["14", "15", "16"] 
    description "PostgreSQL主版本"
    
  param storage: int = 100 
    range [10, 10000] 
    unit "GB" 
    description "数据盘大小"
  
  param ha_enabled: bool = false 
    condition "replica_count>1"  // 开启HA时自动增加副本
) {
  // 隐藏的复杂逻辑 (用户不可见)
  resources = calculate_resources(version, storage)
  
  // 多运行时适配
  deploy docker: {
    image = "postgres:${version}"
    volumes = ["/data:/var/lib/postgresql"]
  }
  
  deploy systemd: {
    service_file = template("""
      [Unit]
      Description=PostgreSQL ${version}
      [Service]
      ExecStart=/usr/bin/postgres -D /data
      LimitNOFILE=65536
      """)
  }
  
  // 自动化运维钩子
  lifecycle {
    pre_install = "apt-get install -y postgresql-${version}"
    health_check = "pg_isready -U postgres"
    backup = "pg_dumpall > /backups/dump.sql"
  }
}
```