# ⚠️ Released at 40 Stars! ⚠️

<img width="800" height="450" alt="ezgif-5e2a950979f4816c" src="https://github.com/user-attachments/assets/b008e081-9329-4523-898e-631801199acc" />


A high-performance, multi-threaded viewbot/solver for guns.lol profiles. It automatically bypasses Cloudflare Turnstile and solves the Proof-of-Work (PoW) challenges using an embedded WASM engine.

## Features
- 🚀 **Multi-threaded:** Run as many concurrent threads as your proxies can handle.
- 🧩 **Auto-Solver:** Solves Turnstile and PoW automatically.
- 🔄 **Proxy Support:** Automatically rotates through your proxies.
- 📊 **Live Stats:** Clean CLI interface with real-time stats in the window title.

---

## 🛠️ Installation

You have two options to run the bot:

### Option 1: Download Compiled Version (Easiest)
1. Go to the **[Releases](../../releases)** page.
2. Download the latest `guns-viewbot.zip`.
3. Extract the ZIP file to a folder.
4. Fill in `solver.txt` and `proxies.txt` (see Configuration below).
5. Run the `.exe` file!

### Option 2: Compile Yourself (For Developers)
If you want to build the bot from the source code, you need to have [Golang](https://go.dev/dl/) installed.
1. Clone this repository or download the source code.
2. Open a terminal/CMD in the folder.
3. Build the executable:
   ```bash
   go build -o guns-viewbot.exe .
   ```
4. Run the generated `guns-viewbot.exe`!

---

## ⚙️ Configuration

Before starting the bot, you need to set up two text files in the same folder as the executable:

1. **`solver.txt`**: Paste your CapMonster API key inside this file. It is required to solve the Turnstile captchas.
2. **`proxies.txt`**: Add your HTTP/HTTPS proxies here (one per line). Format: `http://user:pass@ip:port` or `http://ip:port`. If you leave this empty, the bot will use your direct IP (not recommended for high threads).

---

## 🚀 Usage

Simply double-click the `.exe` or run it from the command line. The bot will ask for:
1. **Username**: The guns.lol profile you want to bot.
2. **Threads**: How many concurrent connections to run. 

Enjoy the views!
