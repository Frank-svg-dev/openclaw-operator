```markdown
# 🦞 OpenClaw Operator

**Kubernetes 原生方式，一键部署 & 生产级运维你的 OpenClaw AI 代理集群**

让 Claude / GPT / Gemini 等大模型真正“活”在你的 Telegram、Discord、Slack、WhatsApp】feishu 里，现在用 Kubernetes 把它们规模化！

[![Go Report Card](https://goreportcard.com/badge/github.com/Frank-svg-dev/openclaw-operator)](https://goreportcard.com/report/github.com/Frank-svg-dev/openclaw-operator)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/Frank-svg-dev/openclaw-operator/blob/main/LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://golang.org)
![Kubernetes](https://img.shields.io/badge/Kubernetes-1.25+-326CE5?logo=kubernetes&logoColor=white)

## 这是什么？

**openclaw-operator** 是为 [OpenClaw](https://github.com/openclaw/openclaw) 量身打造的 Kubernetes Operator。

它让你用最符合 K8s 哲学的方式管理 OpenClaw 实例：

- 一份 YAML → 自动创建 Deployment、Service、ConfigMap、Secret 引用、PVC、NetworkPolicy、PodDisruptionBudget 等全套资源
- 声明式修改配置（模型、API Key、工具开关、消息通道、资源限制、副本数等）
- Operator 自动调谐、修复漂移、处理升级
- 未来支持：自动扩缩容、多租户隔离、监控告警、备份恢复、canary 发布

从单机本地跑 Claw → 企业级多实例 AI 代理农场，一步到位。

## 为什么选择 openclaw-operator？

- **告别手动 YAML 地狱**：不再需要自己写一堆 Deployment + ConfigMap + Ingress
- **深度适配 OpenClaw**：CR 字段直接映射 OpenClaw 的核心配置（model、tools profile、integrations、persistence 等）
- **生产就绪特性**（正在快速迭代中）：
  - 资源 Request/Limit 模板
  - 副本数 & HPA 支持
  - 持久化工作目录（PVC）
  - 网络隔离（NetworkPolicy）
  - 高可用（PDB）
- **极简上手**：CRD 字段语义清晰，新手也能 5 分钟写出第一个实例
- **开源 & 可扩展**：基于 controller-runtime，欢迎贡献新功能

## 快速开始（开发环境）

```bash
# 1. 安装 CRD
make install

# 2. 本地运行 Operator（推荐开发调试）
make run

# 或者直接部署到集群
make deploy IMG=ghcr.io/frank-svg-dev/openclaw-operator:latest

# 3. 创建一个 OpenClaw 实例（示例）
kubectl apply -f - <<EOF
apiVersion: openclaw.io/v1
kind: Openclaw
metadata:
  name: openclaw-agent-test
  namespace: kkk
spec:
  image: 10.29.231.164/ghcr.m.daocloud.io/openclaw/openclaw:latest
  replicas: 1
  serviceType: NodePort
  gatewayPort: 18789
  gatewayBind: lan
  privacy: true
  customApiKey: "sk-3Zxkdrsdf"
  customBaseUrl: "https://cdn.sdf.org/v1"
  customModelId: "qwen3.5-plus"
  customProviderId: "ppnb"
  customCompatibility: openai
  storage:
    accessModes:
      - ReadWriteOnce
    storage: 20Gi
  resources:
    requests:
      cpu: "500m"
      memory: "1Gi"
    limits:
      cpu: "2"
      memory: "4Gi"
EOF
```

几秒后你就可以看到：

```bash
kubectl get all -n ai-agents -l app.kubernetes.io/name=openclaw,app.kubernetes.io/instance=claw-demo
```

## Roadmap（2026 目标）

- [ ] Helm Chart 官方发布
- [ ] OLM / OperatorHub 上架
- [ ] Prometheus ServiceMonitor & Grafana Dashboard
- [ ] 自动升级策略（支持 canary / blue-green）
- [ ] 多租户支持（不同 namespace 不同 Claw 实例隔离）
- [ ] Backup & Restore CR（持久化数据卷快照）
- [ ] 根据消息吞吐量 / 队列长度自动 HPA
- [ ] Sidecar 注入自定义工具容器

## 贡献 & 反馈

欢迎 Issue、PR、Discussion！

尤其欢迎已经在生产环境用 OpenClaw 跑 agent 的同学分享真实痛点和需求。

```bash
# 开发流程快速参考
make generate          # 生成 deepcopy、crd 等
make manifests         # 更新 CRD yaml
make test              # 单元测试
make docker-build      # 构建镜像
make deploy            # 部署到当前集群
```

## 致谢

- [OpenClaw 主项目](https://github.com/openclaw/openclaw) —— 目前最强悍的自托管 AI 代理框架
- Kubebuilder & controller-runtime 社区 —— 让 Operator 开发变得如此丝滑
- 所有在本地/边缘/云上跑 AI agent 的极客们

**让你的 Claw 在 Kubernetes 里自由翱翔吧！**  
🦞🚀

```
