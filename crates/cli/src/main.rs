use clap::{Parser, Subcommand};
use tracing::debug;

/// Warden: 去中心化服务调度管理工具
#[derive(Parser, Debug)]
#[command(name = "warden")]
#[command(author = "你的名字 <you@example.com>")]
#[command(version = "0.1.0")]
#[command(about = "warden", long_about = None)]
struct Cli {
  /// 指定集群节点地址（默认从配置文件加载）
  #[arg(short, long)]
  node: Option<String>,

  /// 启用详细日志输出
  #[arg(short, long, global = true)]
  verbose: bool,

  /// 子命令
  #[command(subcommand)]
  command: Commands,
}

#[derive(Subcommand, Debug)]
enum Commands {
  /// deploy service
  Deploy {
    /// 部署文件路径（yaml/json）
    #[arg(short, long)]
    file: String,
  },
  /// checkout service status
  Status {
    /// 服务名称（不填则显示所有）
    #[arg(short, long)]
    service: Option<String>,
  },
  /// start service
  Start {
    /// service name
    #[arg(short, long)]
    service: String,
  },
  /// stop service
  Stop {
    /// 服务名称
    #[arg(short, long)]
    service: String,
  },
  /// 集群节点管理
  Node {
    #[command(subcommand)]
    action: NodeCommands,
  },
  /// 查看日志
  Logs {
    /// 服务名称
    #[arg(short, long)]
    service: String,
    /// 行数
    #[arg(short, long, default_value_t = 100)]
    lines: usize,
    /// 是否跟踪日志输出
    #[arg(short, long)]
    follow: bool,
  },
}

#[derive(Subcommand, Debug)]
enum NodeCommands {
  /// 列出所有节点
  List,
  /// 查看指定节点详情
  Info {
    /// 节点 ID 或地址
    #[arg(short, long)]
    id: String,
  },
}

fn main() {
  tracing_subscriber::fmt::init();
  let cli = Cli::parse();

  match &cli.command {
    Commands::Deploy { file } => {
      debug!("deploy file path: {}", file);
      // 这里写部署逻辑
    }
    Commands::Status { service } => {
      if let Some(svc) = service {
        println!("查询服务状态: {}", svc);
      } else {
        println!("查询所有服务状态");
      }
    }
    Commands::Start { service } => {
      println!("启动服务: {}", service);
    }
    Commands::Stop { service } => {
      println!("停止服务: {}", service);
    }
    Commands::Node { action } => match action {
      NodeCommands::List => println!("列出所有节点"),
      NodeCommands::Info { id } => println!("节点详情: {}", id),
    },
    Commands::Logs {
      service,
      lines,
      follow,
    } => {
      println!(
        "查看服务日志: {}, 行数: {}, 跟踪: {}",
        service, lines, follow
      );
    }
  }
}
