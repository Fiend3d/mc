# Modal Commander

Modal Commander (mc) is a TUI file manager for Windows (though it might be ported to other platforms in the future). It's heavily inspired by Yazi, Helix, and Total Commander. 

## Demo

![RECLibboard](assets/demo/demo01.png)

## How to Build

Because I fixed a few issues in [github.com/charmbracelet/bubbles](https://github.com/charmbracelet/bubbles), you'll need to get my fork first.

You can do this using:

```powershell
get_forks
```

If that doesn't work, you may need to enable PowerShell scripts first:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

Then simply run:


```powershell
build
```
