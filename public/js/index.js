// Konfigurasi WebSocket
const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
const wsUrl = `${protocol}//${window.location.host}/ws`;
let ws;

// Inisialisasi awal
document.addEventListener("DOMContentLoaded", () => {
    connectWS();
    fetchQueue();
});

// ==========================================
// 1. WEBSOCKET LOGIC (Sinkronisasi Kontrol)
// ==========================================
function connectWS() {
    ws = new WebSocket(wsUrl);

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);

        if (msg.type === 'PLAY') {
            document.getElementById('np-title').innerText = msg.payload.Title;
            fetchQueue(); // Refresh daftar antrian karena ada lagu yang baru naik
        } else if (msg.type === 'QUEUE_EMPTY') {
            document.getElementById('np-title').innerText = "Tidak ada lagu diputar";
            fetchQueue();
        }
        // Perintah lain seperti PAUSE/RESUME tidak perlu merubah UI secara drastis
    };

    ws.onclose = () => {
        setTimeout(connectWS, 3000); // Auto reconnect jika server mati/restart
    };
}

function sendCommand(type, payload = null) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: type, payload: payload }));
    }
}

function changeVolume(val) {
    sendCommand('VOLUME', parseInt(val));
}

// ==========================================
// 2. API LOGIC (Search & Parse URL)
// ==========================================
async function handleSearch() {
    const input = document.getElementById('searchInput').value.trim();
    if (!input) return;

    // Cek apakah input berupa link/URL YouTube
    const isUrl = /^(http|https):\/\/.*(youtube\.com|youtu\.be).*/.test(input);

    let url, options;

    if (isUrl) {
        // Jika URL (termasuk music.youtube.com atau Playlist), panggil endpoint ParseURL
        url = '/api/parse';
        options = {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url: input })
        };
    } else {
        // Jika teks biasa, panggil endpoint Search
        url = `/api/search?q=${encodeURIComponent(input)}`;
        options = { method: 'GET' };
    }

    try {
        document.getElementById('results-section').style.display = 'block';
        document.getElementById('results-list').innerHTML = '<p style="padding: 10px;">Mencari...</p>';

        const response = await fetch(url, options);
        const res = await response.json();

        renderResults(res.data || []);
    } catch (e) {
        alert("Gagal mengambil data dari server");
    }
}

// ==========================================
// 3. RENDER UI & QUEUE MANAGEMENT
// ==========================================
function renderResults(items) {
    const container = document.getElementById('results-list');
    container.innerHTML = '';

    if (items.length === 0) {
        container.innerHTML = '<p style="padding: 10px;">Tidak ditemukan</p>';
        return;
    }

    // Ada kemungkinan yang direturn adalah array (banyak lagu)
    // Jika itu adalah playlist, kita sediakan tombol "Add All" di bagian atas
    if (items.length > 1) {
        const addAllBtn = document.createElement('button');
        addAllBtn.innerText = "Tambahkan Semua ke Antrian";
        addAllBtn.style.width = "100%";
        addAllBtn.style.marginBottom = "10px";
        addAllBtn.onclick = () => addToQueueBulk(items);
        container.appendChild(addAllBtn);
    }

    items.forEach(item => {
        const div = document.createElement('div');
        div.className = 'item';

        // Bungkus hasil stringify dengan fungsi escapeHTML
        const safeItemData = escapeHTML(JSON.stringify(item));

        div.innerHTML = `
            <img src="${item.thumbnail || 'https://placehold.co/50'}" alt="cover">
            <div class="item-info">
                <p class="item-title">${item.title}</p>
                <p class="item-artist">${item.uploader || item.channel || 'Unknown'}</p>
            </div>
            <button class="action-btn add" onclick='addToQueueSingle(${safeItemData})'>+</button>
        `;
        container.appendChild(div);
    });
}

// Fungsi bantu untuk menambahkan satu lagu
function addToQueueSingle(item) {
    addToQueueBulk([item]);
}

// Mengirim data lagu (bisa 1, bisa 100 jika playlist) ke API
async function addToQueueBulk(itemsArray) {
    // Format struktur sesuai dengan models.Queue di Golang
    const payloadItems = itemsArray.map(item => ({
        VideoID: item.id,
        Title: item.title,
        Artist: item.uploader || item.channel,
        Duration: item.duration || 0,
        Thumbnail: item.thumbnail
    }));

    try {
        await fetch('/api/queue', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ items: payloadItems })
        });

        // Bersihkan hasil pencarian dan refresh antrian
        document.getElementById('searchInput').value = '';
        document.getElementById('results-section').style.display = 'none';

        fetchQueue();
    } catch (e) {
        alert("Gagal menambahkan ke antrian");
    }
}

async function fetchQueue() {
    try {
        const response = await fetch('/api/queue');
        const res = await response.json();

        const container = document.getElementById('queue-list');
        container.innerHTML = '';

        if (!res.data || res.data.length === 0) {
            container.innerHTML = '<p style="padding: 10px; color: gray;">Antrian kosong</p>';
            return;
        }

        currentWaitingQueue = res.data.filter(song => song.Status === 'waiting');

        res.data.forEach(song => {
            const div = document.createElement('div');
            div.className = 'item';

            if (song.Status === 'playing') {
                div.style.borderLeft = "3px solid #1db954";
                div.style.backgroundColor = "#2a2a2a";
            }

            // Merakit tombol aksi dinamis
            let actionButtons = '';
            if (song.Status === 'waiting') {
                const waitIdx = currentWaitingQueue.findIndex(s => s.ID === song.ID);

                // TOMBOL PLAY NOW BARU
                actionButtons += `<button class="action-btn move" onclick="playNow(${song.ID})" style="border-color: #1db954; color: #1db954; font-weight: bold;">▶</button>`;

                if (waitIdx > 0) {
                    actionButtons += `<button class="action-btn move" onclick="moveQueue(${waitIdx}, -1)">▲</button>`;
                }
                if (waitIdx < currentWaitingQueue.length - 1) {
                    actionButtons += `<button class="action-btn move" onclick="moveQueue(${waitIdx}, 1)">▼</button>`;
                }
                actionButtons += `<button class="action-btn delete" onclick="deleteQueue('${song.ID}')">X</button>`;
            }

            div.innerHTML = `
                <img src="${song.Thumbnail || 'https://via.placeholder.com/50'}" alt="cover">
                <div class="item-info">
                    <p class="item-title">${song.Title}</p>
                    <p class="item-artist">${song.Artist} ${song.Status === 'playing' ? '(Now Playing)' : ''}</p>
                </div>
                <div style="display: flex; gap: 5px;">
                    ${actionButtons}
                </div>
            `;
            container.appendChild(div);
        });
    } catch (e) {
        console.error("Gagal mengambil antrian:", e);
    }
}

async function deleteQueue(id) {
    await fetch(`/api/queue/${id}`, { method: 'DELETE' });
    fetchQueue();
}

async function clearQueue() {
    const confirmClear = confirm("Yakin ingin menghapus semua antrian?");
    if (!confirmClear) return;

    try {
        await fetch('/api/queue/clear', { method: 'DELETE' });
        fetchQueue(); // Refresh daftar UI setelah dihapus
    } catch (e) {
        console.error("Gagal membersihkan antrian:", e);
    }
}

// Fungsi Refresh dengan Indikator UI
async function manualRefresh() {
    const btn = document.getElementById('btn-refresh');
    const indicator = document.getElementById('refresh-indicator');

    // 1. Tampilkan indikator & matikan tombol sementara
    btn.disabled = true;
    btn.style.opacity = '0.5';
    indicator.style.display = 'inline';

    // 2. Tarik data terbaru dari server
    await fetchQueue();

    // 3. Sembunyikan indikator & hidupkan tombol lagi (diberi jeda dikit agar transisi terlihat mulus)
    setTimeout(() => {
        indicator.style.display = 'none';
        btn.disabled = false;
        btn.style.opacity = '1';
    }, 400);
}

let currentWaitingQueue = []; // Variabel untuk menyimpan urutan lagu berstatus 'waiting'

async function moveQueue(index, direction) {
    // Tukar posisi elemen di dalam array Javascript
    const temp = currentWaitingQueue[index];
    currentWaitingQueue[index] = currentWaitingQueue[index + direction];
    currentWaitingQueue[index + direction] = temp;

    // Ambil array ID yang sudah berurutan dengan posisi baru
    const newOrderIDs = currentWaitingQueue.map(song => song.ID);

    try {
        // Tembak susunan ID baru ke API
        await fetch('/api/queue/reorder', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ids: newOrderIDs })
        });

        fetchQueue(); // Refresh UI agar sinkron dengan database
    } catch (e) {
        console.error("Gagal mengubah urutan:", e);
    }
}

// ==========================================
// 4. SETTINGS & PLAY NOW LOGIC
// ==========================================
async function openSettings() {
    document.getElementById('settings-modal').style.display = 'flex';
    try {
        const res = await fetch('/api/config');
        const data = await res.json();

        if (data.data.audio_codec) document.getElementById('set-codec').value = data.data.audio_codec;
        if (data.data.audio_bitrate) document.getElementById('set-bitrate').value = data.data.audio_bitrate;
    } catch (e) {
        console.error("Gagal memuat pengaturan");
    }
}

function closeSettings() {
    document.getElementById('settings-modal').style.display = 'none';
}

async function saveSettings() {
    const payload = {
        audio_codec: document.getElementById('set-codec').value,
        audio_bitrate: document.getElementById('set-bitrate').value
    };

    try {
        await fetch('/api/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        closeSettings();
    } catch (e) {
        alert("Gagal menyimpan pengaturan");
    }
}

function playNow(id) {
    if (confirm("Mainkan lagu ini sekarang? (Lagu saat ini akan dihentikan)")) {
        sendCommand('PLAY_NOW', id);
        // Kita beri efek visual sesaat sebelum refresh UI
        setTimeout(fetchQueue, 1000);
    }
}

// Fungsi untuk menjinakkan karakter sensitif HTML
function escapeHTML(str) {
    return str
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;'); // &#39; lebih universal dan stabil dibanding &apos;
}