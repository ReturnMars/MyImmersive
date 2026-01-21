// MyImmersive 设置页面逻辑

// 默认配置
const DEFAULT_CONFIG = {
  targetLang: 'zh-CN',
  apiUrl: 'http://localhost:8080',
  autoTranslate: false
};

// DOM 元素
const form = document.getElementById('settings-form');
const targetLangSelect = document.getElementById('targetLang');
const apiUrlInput = document.getElementById('apiUrl');
const autoTranslateCheckbox = document.getElementById('autoTranslate');
const statusDiv = document.getElementById('status');

// 加载配置
async function loadConfig() {
  try {
    const result = await chrome.storage.sync.get(DEFAULT_CONFIG);
    targetLangSelect.value = result.targetLang;
    apiUrlInput.value = result.apiUrl;
    autoTranslateCheckbox.checked = result.autoTranslate;
  } catch (err) {
    console.error('[Popup] Failed to load config:', err);
  }
}

// 保存配置
async function saveConfig(e) {
  e.preventDefault();
  
  const config = {
    targetLang: targetLangSelect.value,
    apiUrl: apiUrlInput.value.trim() || DEFAULT_CONFIG.apiUrl,
    autoTranslate: autoTranslateCheckbox.checked
  };

  try {
    await chrome.storage.sync.set(config);
    showStatus('✅ 设置已保存');
    console.log('[Popup] Config saved:', config);
  } catch (err) {
    showStatus('❌ 保存失败');
    console.error('[Popup] Failed to save config:', err);
  }
}

// 显示状态
function showStatus(message) {
  statusDiv.textContent = message;
  setTimeout(() => {
    statusDiv.textContent = '';
  }, 2000);
}

// 初始化
document.addEventListener('DOMContentLoaded', loadConfig);
form.addEventListener('submit', saveConfig);
