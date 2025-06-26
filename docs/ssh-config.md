
```bash
ssh-keygen -t ed25519 -C "kauri.kim@gmail.com" -f ~/.ssh/project_deploy_key
```

```bash
git remote set-url origin git@github.com-project:kaurikim/project.git
```

```bash
Host github.com-project
    HostName github.com
    User git
    IdentityFile ~/.ssh/project_deploy_key
    IdentitiesOnly yes
```
