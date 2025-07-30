VAGRANT_NODES = 3

Vagrant.configure("2") do |config|
  config.vm.box = "generic/ubuntu2204"  # 可以换成你喜欢的 box

  (1..VAGRANT_NODES).each do |i|
    config.vm.define "node#{i}" do |node|
      node.vm.hostname = "node#{i}"
      node.vm.provider "hyperv" do |hv|
        hv.memory = 512
        hv.cpus = 1
        hv.vmname = "raft-node#{i}"
      end
      node.vm.network "private_network", ip: "192.168.56.#{100+i}"
      node.vm.provision "shell", inline: <<-SHELL
        echo "Node #{i} provisioned."
        sudo apt-get update
        sudo apt-get install -y curl unzip
      SHELL
    end
  end
end
