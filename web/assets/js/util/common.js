const ONE_KB = 1024;
const ONE_MB = ONE_KB * 1024;
const ONE_GB = ONE_MB * 1024;
const ONE_TB = ONE_GB * 1024;
const ONE_PB = ONE_TB * 1024;

function sizeFormat(size) {
    if (size < 0) {
        return "0 B";
    } else if (size < ONE_KB) {
        return size.toFixed(0) + " B";
    } else if (size < ONE_MB) {
        return (size / ONE_KB).toFixed(2) + " KB";
    } else if (size < ONE_GB) {
        return (size / ONE_MB).toFixed(2) + " MB";
    } else if (size < ONE_TB) {
        return (size / ONE_GB).toFixed(2) + " GB";
    } else if (size < ONE_PB) {
        return (size / ONE_TB).toFixed(2) + " TB";
    } else {
        return (size / ONE_PB).toFixed(2) + " PB";
    }
}

function cpuSpeedFormat(speed) {
    if (speed > 1000) {
        const GHz = speed / 1000;
        return GHz.toFixed(2) + " GHz";
    } else {
        return speed.toFixed(2) + " MHz";
    }
}

function cpuCoreFormat(cores) {
    if (cores === 1) {
        return "1 Core";
    } else {
        return cores + " Cores";
    }
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
    if (second < 60) {
        return second.toFixed(0) + ' s';
    } else if (second < 3600) {
        return (second / 60).toFixed(0) + ' m';
    } else if (second < 3600 * 24) {
        return (second / 3600).toFixed(0) + ' h';
    } else {
        return (second / 3600 / 24).toFixed(0) + ' d';
    }
}

function addZero(num) {
    if (num < 10) {
        return "0" + num;
    } else {
        return num;
    }
}

function toFixed(num, n) {
    n = Math.pow(10, n);
    return Math.round(num * n) / n;
}

function debounce(fn, delay) {
    var timeoutID = null;
    return function () {
        clearTimeout(timeoutID);
        var args = arguments;
        var that = this;
        timeoutID = setTimeout(function () {
            fn.apply(that, args);
        }, delay);
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
    switch (true) {
        case data === null:
            return 'blue';
        case total <= 0:
            return 'blue';
        case data < total - threshold:
            return 'cyan';
        case data < total:
            return 'orange';
        default:
            return 'red';
    }
}

function doAllItemsExist(array1, array2) {
    for (let i = 0; i < array1.length; i++) {
        if (!array2.includes(array1[i])) {
            return false;
        }
    }
    return true;
}

function buildURL({ host, port, isTLS, base, path }) {
    if (!host || host.length === 0) host = window.location.hostname;
    if (!port || port.length === 0) port = window.location.port;

    if (isTLS === undefined) isTLS = window.location.protocol === "https:";

    const protocol = isTLS ? "https:" : "http:";

    port = String(port);
    if (port === "" || (isTLS && port === "443") || (!isTLS && port === "80")) {
        port = "";
    } else {
        port = `:${port}`;
    }

    return `${protocol}//${host}${port}${base}${path}`;
}
