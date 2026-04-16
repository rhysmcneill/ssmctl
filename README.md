# ssmctl

A lightweight CLI for managing AWS SSM connections, remote command execution, and file transfers — designed to feel like a modern SSH/SCP replacement powered by AWS Systems Manager.

---

## 🚧 Status

**Work in progress.**

This project is currently under active development. The features below describe the intended functionality for the first version (v1), but they may not be fully implemented yet.

---

## 🎯 Planned Features (v1)

### Connect to an instance

```bash
ssmctl connect <target>
```

---

### Run a command

```bash
ssmctl run <target> -- <command>
```

Example:

```bash
ssmctl run web-1 -- uname -a
```

---

### Upload a file

```bash
ssmctl cp ./file.txt <target>:/tmp/file.txt
```

---

### Download a file

```bash
ssmctl cp <target>:/var/log/app.log ./app.log
```

---

### Show version

```bash
ssmctl version
```

---

## 🎯 Targets

A `<target>` will support:

- EC2 instance ID  
  `i-0123456789abcdef0`

- Instance Name tag  
  `web-1`

Resolution strategy:

- If input starts with `i-` → treated as instance ID  
- Otherwise → resolved via EC2 Name tag  

---

## ⚙️ Planned Global Flags

```bash
--profile, -p   AWS profile (defaults to AWS_PROFILE)
--region, -r    AWS region
--output, -o    Output format: text | json
--debug, -d     Enable debug logging
--timeout, -t   Command timeout
```

---

## 🧠 Design Goals

- Simple, ergonomic CLI (inspired by `ssh` and `scp`)
- No need for SSH keys or open ports
- Built on AWS SSM
- Works with existing AWS credentials/config
- Scriptable (`--output json`)

---

## 🏗 Planned Project Structure

```text
ssmctl/
├── cmd/ssmctl/main.go
├── internal/
│   ├── cmd/
│   ├── app/
│   ├── config/
│   ├── ssm/
│   ├── output/
│   └── version/
├── go.mod
└── go.sum
```

---

## 📌 Roadmap

- [ ] `connect` via SSM session manager  
- [ ] `run` command execution via `SendCommand`  
- [ ] `cp` upload (local → remote)  
- [ ] `cp` download (remote → local)  
- [ ] target resolution (instance ID + Name tag)  
- [ ] structured output (`text` / `json`)  
- [ ] timeout + context handling  
- [ ] basic error handling and validation  

---

## 🤝 Contributing

Not open for contributions yet — project is still being shaped.

---

## 📄 License

MIT License