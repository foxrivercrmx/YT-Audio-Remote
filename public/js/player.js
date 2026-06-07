const audio = document.getElementById('audio-player');
const coverEl = document.getElementById('cover');
const titleEl = document.getElementById('title');
const artistEl = document.getElementById('artist');
const progressBar = document.getElementById('progress-bar');
const startOverlay = document.getElementById('start-overlay');

const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
const wsUrl = `${protocol}//${window.location.host}/ws`;

let ws;
let isInitialized = false;

// Fungsi ini dipanggil dari onclick overlay atau tombol Enter di remote STB
function initPlayer() {
    if (isInitialized) return;
    isInitialized = true;

    // 1. Sembunyikan overlay
    startOverlay.style.display = 'none';

    // 2. Pancing engine audio browser agar terbuka (unlock autoplay)
    audio.play().catch(() => { });
    audio.pause();

    // 3. Setelah aman, baru kita hubungkan ke server
    connectWS();
}

// Menangkap tombol OK/Enter fisik dari Remote STB b860h
document.addEventListener('keydown', (e) => {
    // Remote STB biasanya mengirimkan event key 'Enter' saat tombol tengah (OK) ditekan
    if (e.key === 'Enter' && !isInitialized) {
        initPlayer();
    }
});

function connectWS() {
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        console.log("Player terhubung ke server WebSocket.");
        // Opsional: Bisa kirim sinyal ke server bahwa player sudah siap menerima lagu
    };

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);

        switch (msg.type) {
            case 'PLAY':
                const song = msg.payload;

                titleEl.innerText = song.Title;
                artistEl.innerText = song.Artist || song.Uploader;

                if (song.Thumbnail) {
                    coverEl.src = song.Thumbnail;
                }

                audio.src = song.DirectURL;
                // Karena initPlayer() sudah dijalankan, audio.play() di sini 
                // dijamin 100% jalan tanpa diblokir browser.
                audio.play();
                break;

            case 'PAUSE':
                audio.pause();
                break;

            case 'RESUME':
                audio.play();
                break;

            case 'VOLUME':
                audio.volume = msg.payload / 100;
                break;

            case 'QUEUE_EMPTY':
                titleEl.innerText = "Antrian Habis";
                artistEl.innerText = "-";
                coverEl.src = "https://placehold.co/300x300?text=Antrian+Habis";
                progressBar.style.width = "0%";
                break;
        }
    };

    ws.onclose = () => {
        console.log("Koneksi terputus. Mencoba reconnect dalam 3 detik...");
        setTimeout(connectWS, 3000);
    };
}

// Logika perputaran lagu dan progress bar
audio.addEventListener('ended', () => {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'SONG_ENDED' }));
    }
});

// Tangkap jika terjadi error pada link stream (misal: expired, koneksi putus)
audio.addEventListener('error', (e) => {
    console.error("Terjadi error pada pemutaran stream. Melompat ke lagu berikutnya...", e);
    // Anggap saja lagu selesai agar backend langsung memutar antrian selanjutnya
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'SONG_ENDED' }));
    }
});

// (Opsional) Tangkap jika pemutaran macet total karena tidak bisa buffering
audio.addEventListener('stalled', () => {
    console.warn("Audio stalled/macet. Menunggu jaringan...");
    // Kamu bisa bereksperimen melempar SONG_ENDED juga di sini jika dirasa 
    // stalled-nya tidak pernah bisa pulih. Tapi untuk sekarang kita log saja.
});

audio.addEventListener('timeupdate', () => {
    if (audio.duration) {
        const percentage = (audio.currentTime / audio.duration) * 100;
        progressBar.style.width = percentage + "%";
    }
});