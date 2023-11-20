const ONE_KB = 1024;
const ONE_MB = ONE_KB * 1024;
const ONE_GB = ONE_MB * 1024;
const ONE_TB = ONE_GB * 1024;
const ONE_PB = ONE_TB * 1024;

function sizeFormat(size) {
    const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    let index = 0;
    while (size >= 1024 && index < units.length - 1) {
        size /= 1024;
        index++;
    }
    return size.toFixed(index ? 2 : 0) + ' ' + units[index];
}

function cpuSpeedFormat(speed) {
    return (speed > 1000 ? (speed / 1000).toFixed(2) + " GHz" : speed.toFixed(2) + " MHz");
}

function cpuCoreFormat(cores) {
    return cores + (cores === 1 ? " Core" : " Cores");
}

function base64(str) {
    return Base64.encode(str);
}

function safeBase64(str) {
    return base64(str)
        .replace(/\+/g, '-')
        .replace(/=/g, '')
        .replace(/\//g, '_');
}

function formatSecond(second) {
    const timeUnits = { day: 86400, hour: 3600, minute: 60 };
    for (const [unit, value] of Object.entries(timeUnits)) {
        if (second >= value) {
            return (second / value).toFixed(0) + ' ' + unit.charAt(0);
        }
    }
    return second.toFixed(0) + ' s';
}

function addZero(num) {
    return num < 10 ? "0" + num : num.toString();
}

function toFixed(num, n) {
    const factor = Math.pow(10, n);
    return Math.round(num * factor) / factor;
}

function debounce(fn, delay) {
    let timeoutID = null;
    return function (...args) {
        clearTimeout(timeoutID);
        timeoutID = setTimeout(() => fn.apply(this, args), delay);
    };
}

function getCookie(cname) {
    let name = cname + '=';
    let ca = document.cookie.split(';');
    for (let i = 0; i < ca.length; i++) {
        let c = ca[i];
        while (c.charAt(0) == ' ') {
            c = c.substring(1);
        }
        if (c.indexOf(name) == 0) {
            // decode cookie value only
            return decodeURIComponent(c.substring(name.length, c.length));
        }
    }
    return '';
}


function setCookie(cname, cvalue, exdays) {
    const d = new Date();
    d.setTime(d.getTime() + exdays * 24 * 60 * 60 * 1000);
    let expires = 'expires=' + d.toUTCString();
    // encode cookie value
    document.cookie = cname + '=' + encodeURIComponent(cvalue) + ';' + expires + ';path=/';
}

function usageColor(data, threshold, total) {
    if (data === null || total <= 0) return 'blue';
    if (data < total - threshold) return 'cyan';
    if (data < total) return 'orange';
    return 'red';
}

function doAllItemsExist(array1, array2) {
    return array1.every(item => array2.includes(item));
}

function buildURL({ host = window.location.hostname, port = window.location.port, isTLS = window.location.protocol === "https:", base, path }) {
    const protocol = isTLS ? "https:" : "http:";
    port = String(port);
    port = (port === "" || (isTLS && port === "443") || (!isTLS && port === "80")) ? "" : `:${port}`;
    return `${protocol}//${host}${port}${base}${path}`;
}
