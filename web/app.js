// Tenchi Geolocation App JS
let map, marker;
let isAuthenticated = false;
let geoMarkers = [];

window.onload = function() {
  // 地図初期化
  map = L.map('map').setView([35.681236, 139.767125], 13); // 東京駅
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '© OpenStreetMap contributors'
  }).addTo(map);
  map.invalidateSize();

  // 現在地取得
  if (navigator.geolocation) {
    navigator.geolocation.getCurrentPosition(pos => {
      const lat = pos.coords.latitude;
      const lng = pos.coords.longitude;
      map.setView([lat, lng], 16);
      marker = L.marker([lat, lng]).addTo(map).bindPopup('あなたの現在地');
    });
  }

  // ドロワーメニュー開閉
  const drawer = document.getElementById('drawer');
  const openBtn = document.getElementById('drawer-open-btn');
  const closeBtn = document.getElementById('drawer-close-btn');
  const geoBtn = document.getElementById('register-geo-btn');
  const updateBtn = document.getElementById('update-geo-btn');

  // 初回ロード時に認証状態を判定
  fetch('/api/user/me').then(res => {
    if (res.status === 401 || res.status === 403) {
      isAuthenticated = false;
      geoBtn.classList.add('unauth');
    } else {
      isAuthenticated = true;
      geoBtn.classList.remove('unauth');
    }
  });

  geoBtn.onclick = () => {
    if (!isAuthenticated) {
      alert('ログインしてください');
      return;
    }
    if (!marker) return alert('現在地が取得できていません');
    const {lat, lng} = marker.getLatLng();
    fetch('/api/geo', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({latitude: lat, longitude: lng})
    }).then(res => {
      if (res.status === 403) {
        return res.text().then(msg => {
          if (msg.includes('Request not allowed at this time')) {
            alert('現在は登録できません（指定された時間外です）');
            return;
          }
          alert('登録できません: ' + msg);
        });
      }
      if (!res.ok) {
        return res.text().then(msg => alert('登録できません: ' + msg));
      }
      return res.json();
    }).then(data => {
      if (data && data.latitude) {
        alert('位置情報を登録しました');
      }
    });
  };

  // --- 位置情報マーカーを全てクリア ---
  function clearGeoMarkers() {
    geoMarkers.forEach(m => map.removeLayer(m));
    geoMarkers = [];
  }

  // --- /api/geo で全チームの位置を取得しマップに描画 ---
  function fetchAndShowAllTeamsGeo() {
    let myTeamId = null;
    // まず自分のチームIDを取得
    fetch('/api/user/me').then(res => {
      if (!res.ok) throw new Error('ユーザー情報の取得に失敗しました (ログインしていますか？)');
      return res.json();
    }).then(userData => {
      myTeamId = userData.team.id || userData.team.ID || null;
      return fetch('/api/geo');
    }).then(res => {
      if (!res.ok) throw new Error('位置情報の取得に失敗しました');
      return res.json();
    }).then(data => {
      clearGeoMarkers();
      if (!Array.isArray(data)) return;
      data.forEach(detail => {
        const geo = detail.Geolocation || detail.geolocation;
        const team = (detail.TeamDetail || detail.team_detail || detail).Team || detail.team;
        if (!geo || !team) return;
        const lat = geo.Latitude || geo.latitude;
        const lng = geo.Longitude || geo.longitude;
        const teamName = team.Name || team.name || 'チーム';
        const teamId = team.ID || team.id;
        const time = geo.CreatedAt || geo.created_at || '';
        // マーカー色: 自分のチームは青, 他は緑
        let markerColor = (myTeamId && teamId && String(myTeamId) === String(teamId)) ? '#3498db' : '#27ae60';
        const icon = L.divIcon({
          className: '',
          html: `<div style="background:${markerColor};width:22px;height:22px;border-radius:50%;border:2px solid #fff;box-shadow:0 2px 6px #0002;"></div>`,
          iconSize: [22,22],
          iconAnchor: [11,11],
        });
        const m = L.marker([lat, lng], {icon}).addTo(map)
          .bindPopup(`<b>${teamName}</b><br>(${lat.toFixed(5)}, ${lng.toFixed(5)})<br>${time ? '更新: '+time : ''}`);
        geoMarkers.push(m);
      });
      setMapUpdateTime();
    }).catch(e => {
      alert(e.message || '位置情報の取得に失敗しました');
    });
  }

  function setMapUpdateTime() {
    const el = document.getElementById('map-update-time');
    if (!el) return;
    const now = new Date();
    const y = now.getFullYear();
    const m = (now.getMonth()+1).toString().padStart(2,'0');
    const d = now.getDate().toString().padStart(2,'0');
    const h = now.getHours().toString().padStart(2,'0');
    const min = now.getMinutes().toString().padStart(2,'0');
    const s = now.getSeconds().toString().padStart(2,'0');
    el.textContent = `マップ最終更新: ${y}/${m}/${d} ${h}:${min}:${s}`;
  }

  if (updateBtn) {
    updateBtn.onclick = () => {
      fetchAndShowAllTeamsGeo();
    };
  }

  // ドロワーメニュー開閉
  if (openBtn && drawer) {
    openBtn.onclick = (e) => {
      e.stopPropagation();
      drawer.classList.add('open');
      // ドロワーを開くたびにユーザ情報を取得・描画
      fetch('/api/user/me').then(res => {
        if (res.status === 401 || res.status === 403) {
          // 認証エラー時はログインボタンのみ表示
          document.querySelector('.drawer-content').innerHTML = `
            <div style="display:flex;flex-direction:column;align-items:center;gap:1.5em;justify-content:center;height:100%;width:100%;">
              <button id="login-btn" class="drawer-login-btn">ログイン</button>
            </div>
          `;
          document.getElementById('login-btn').onclick = () => {
            window.location.href = '/api/login';
          };
          return;
        }
        return res.json();
      }).then(data => {
        if (!data) return;
        // プロフィール
        const teamNameElem = document.getElementById('drawer-profile-team');
        teamNameElem.textContent = data.team.name || data.team.Name || '';
        teamNameElem.style.cursor = 'pointer';
        teamNameElem.title = 'タップしてチーム名を変更';
        teamNameElem.onclick = () => {
          const current = teamNameElem.textContent;
          const newName = prompt('新しいチーム名を入力してください', current);
          if (newName && newName !== current) {
            fetch(`/api/team/${data.team.id || data.team.ID}`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ name: newName })
            })
            .then(res => {
              if (!res.ok) throw new Error('チーム名の変更に失敗しました');
              return res.json();
            })
            .then(team => {
              teamNameElem.textContent = team.name || team.Name || newName;
              alert('チーム名を変更しました');
            })
            .catch(e => alert(e.message || 'チーム名の変更に失敗しました'));
          }
        };
        document.getElementById('drawer-profile-avatar').src = data.user_profile.avatar_url || data.user_profile.AvatarURL || 'https://www.gravatar.com/avatar/?d=mp';
        const userNameElem = document.getElementById('drawer-profile-username');
        userNameElem.textContent = data.user_profile.user_name || data.user_profile.UserName || '';
        userNameElem.style.cursor = 'pointer';
        userNameElem.title = 'タップしてユーザ名を変更';
        userNameElem.onclick = () => {
          const current = userNameElem.textContent;
          const newName = prompt('新しいユーザ名を入力してください', current);
          if (newName && newName !== current) {
            fetch('/api/user/me/name', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ name: newName })
            })
            .then(res => {
              if (!res.ok) throw new Error('ユーザ名の変更に失敗しました');
              return res.json();
            })
            .then(profile => {
              userNameElem.textContent = profile.user_name || profile.UserName || newName;
              alert('ユーザ名を変更しました');
            })
            .catch(e => alert(e.message || 'ユーザ名の変更に失敗しました'));
          }
        };
        // チームメンバー
        const users = data.team_members || [];
        const ul = document.getElementById('drawer-team-users');
        ul.innerHTML = '';
        users.forEach(member => {
          const li = document.createElement('li');
          li.className = 'drawer-team-user';
          li.innerHTML = `
            <img class="drawer-team-user-avatar" src="${member.avatar_url || member.AvatarURL || 'https://www.gravatar.com/avatar/?d=mp'}" alt="avatar" />
            <span class="drawer-team-user-name">${member.user_name || member.UserName || ''}</span>
          `;
          ul.appendChild(li);
        });
      });
    };
  }
  if (closeBtn && drawer) {
    closeBtn.onclick = (e) => {
      e.stopPropagation();
      drawer.classList.remove('open');
    };
  }
  if (drawer) {
    drawer.addEventListener('click', e => {
      if (e.target === drawer) {
        drawer.classList.remove('open');
      }
    });
  }
};

function deleteAllSiteCookies() {
    const cookies = document.cookie.split(';');
    for (let cookie of cookies) {
        const eqPos = cookie.indexOf('=');
        const name = eqPos > -1 ? cookie.substr(0, eqPos).trim() : cookie.trim();
        // ドメインとパスを考慮して削除
        document.cookie = `${name}=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/`;
        document.cookie = `${name}=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/;domain=${location.hostname}`;
    }
}
