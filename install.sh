#!/bin/bash
# Installer Otomatis YT Audio Remote (Malas & Hemat Edition)

set -e

echo "===================================================="
echo "  Mulai Instalasi YT Audio Remote & Dependencies..."
echo "===================================================="

if [ "$EUID" -ne 0 ]; then
  echo "❌ Tolong jalankan pakai sudo ya Kang! (sudo bash install.sh)"
  exit
fi

echo "📦 Menginstall FFmpeg & alat tukang..."
apt-get update -y
apt-get install -y curl wget ffmpeg build-essential git

if ! command -v qjs &> /dev/null; then
    echo "🪶 Merakit QuickJS dari source..."
    rm -rf /tmp/quickjs
    git clone https://github.com/bellard/quickjs.git /tmp/quickjs
    cd /tmp/quickjs
    make
    make install
    ln -sf /usr/local/bin/qjs /usr/bin/quickjs
    ln -sf /usr/local/bin/qjs /usr/local/bin/quickjs
    cd ~
else
    echo "✅ QuickJS sudah terinstall!"
fi

if ! command -v yt-dlp &> /dev/null; then
    echo "📥 Menginstall yt-dlp..."
    wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -O /usr/local/bin/yt-dlp
    chmod a+rx /usr/local/bin/yt-dlp
else
    echo "✅ YT-dlp sudah terinstall!"
fi

echo "🚀 Mengambil binary YT Audio Remote..."
mkdir -p /opt/yt-audio-remote
cd /opt/yt-audio-remote
mkdir -p /opt/yt-audio-remote/data

YTAUDIO_URL="https://gitlab.com/denx.bluemonday/yt-audio-remote/-/jobs/artifacts/main/raw/yt-audio-remote?job=build_goke" 
wget -q --show-progress "$YTAUDIO_URL" -O yt-audio-remote
chmod +x yt-audio-remote
mv yt-audio-remote /usr/local/bin/

touch /opt/yt-audio-remote/cookies.txt
touch /opt/yt-audio-remote/yt-audio.db

echo "⚙️ Membuat Systemd Service..."
cat <<EOF > /etc/systemd/system/yt-audio-remote.service
[Unit]
Description=YT Audio Remote Backend Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/yt-audio-remote
ExecStart=/usr/local/bin/yt-audio-remote -db=/opt/yt-audio-remote/yt-audio.db -port=5710 -js-runtimes=quickjs -cookies=/opt/yt-audio-remote/cookies.txt
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

echo "🔥 Menyalakan YT Audio Remote Backend"
systemctl daemon-reload
systemctl enable yt-audio-remote
systemctl restart yt-audio-remote

echo "===================================================="
echo "  WES BERES KANG! YT Audio Remote Backend sudah jalan di background."
echo "  Cek status: systemctl status yt-audio-remote"
echo "  Cek log: journalctl -fu yt-audio-remote"
echo "===================================================="