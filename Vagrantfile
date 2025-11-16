Vagrant.configure("2") do |config|
  # 基础镜像
  config.vm.box = "ubuntu/jammy64"

  # 通用配置
  config.vm.provider "virtualbox" do |vb|
    vb.memory = 1024
    vb.maxmemory = 2048
    vb.cpus = 2
  end

  # 自定义网络
  # 每个节点有一个内网 IP，用于 raft 节点间通信
  nodes = [
    { :name => "node1", :ip => "192.168.56.11" },
    { :name => "node2", :ip => "192.168.56.12" },
    { :name => "node3", :ip => "192.168.56.13" }
  ]

  nodes.each do |node|
    config.vm.define node[:name] do |n|
      n.vm.hostname = node[:name]
      n.vm.network "private_network", ip: node[:ip]

      # 安装 Docker
      n.vm.provision "shell", inline: <<-SHELL
        apt-get update -y
        apt-get install -y ca-certificates curl gnupg lsb-release
        mkdir -p /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        echo \
          "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
          https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
          > /etc/apt/sources.list.d/docker.list
        apt-get update -y
        apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
        usermod -aG docker vagrant
        systemctl enable docker
      SHELL
    end
  end
end
