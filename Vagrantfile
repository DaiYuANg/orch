Vagrant.configure("2") do |config|
  # Use the same box for all nodes by default. You can override via ORCH_VAGRANT_BOX.
  config.vm.box = ENV.fetch("ORCH_VAGRANT_BOX", "ubuntu/jammy64")

  # Shared topology for local multi-node smoke / raft scenarios.
  nodes = [
    { name: "node1", ip: "192.168.56.11" },
    { name: "node2", ip: "192.168.56.12" },
    { name: "node3", ip: "192.168.56.13" }
  ]

  config.vm.provider "virtualbox" do |vb|
    vb.name = "orch-vagrant-cluster"
    vb.memory = 1024
    vb.maxmemory = 2048
    vb.cpus = 2
    vb.customize ["modifyvm", :id, "--natdnshostresolver1", "on"]
  end

  nodes.each_with_index do |node, index|
    config.vm.define node[:name] do |n|
      n.vm.hostname = node[:name]
      n.vm.network "private_network", ip: node[:ip]
      n.vm.network "forwarded_port", guest: 17443, host: 17443 + index
      n.vm.network "forwarded_port", guest: 17451 + index, host: 17451 + index
      n.vm.synced_folder ".", "/vagrant", type: "virtualbox"

      n.vm.provision "shell", path: "scripts/vagrant/bootstrap-node.sh",
        privileged: true,
        env: {
          "ORCH_VAGRANT_DOCKER_CHANNEL" => ENV.fetch("ORCH_VAGRANT_DOCKER_CHANNEL", "stable"),
          "ORCH_VAGRANT_DOCKER_ARCH" => ENV.fetch("ORCH_VAGRANT_DOCKER_ARCH", "")
        }
    end
  end
end
