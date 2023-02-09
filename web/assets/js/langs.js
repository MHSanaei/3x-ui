supportLangs = [
    {
       name : "English",
       value : "en-US",
       icon : "🇺🇸"
    },
	{
        name : "Farsi",
        value : "fa_IR",
        icon : "🇮🇷"
    },
    {
        name : "汉语",
        value : "zh-Hans",
        icon : "🇨🇳"
    },
]

function getLang(){
    let lang = getCookie('lang')

    if (! lang){
        if (window.navigator){
            lang = window.navigator.language || window.navigator.userLanguage;

            if (isSupportLang(lang)){
                setCookie('lang' , lang , 150)
            }else{
                setCookie('lang' , 'en-US' , 150)
                window.location.reload();
            }
        }else{
            setCookie('lang' , 'en-US' , 150)
            window.location.reload();
        }
    }

    return lang;
}

function setLang(lang){

    if (!isSupportLang(lang)){
        lang = 'en-US';
    }

    setCookie('lang' , lang , 150)
    window.location.reload();
}

function isSupportLang(lang){
    for (l of supportLangs){
        if (l.value === lang){
            return true;
        }
    }

    return false;
}



function getCookie(cname) {
    let name = cname + "=";
    let decodedCookie = decodeURIComponent(document.cookie);
    let ca = decodedCookie.split(';');
    for(let i = 0; i <ca.length; i++) {
        let c = ca[i];
        while (c.charAt(0) == ' ') {
            c = c.substring(1);
        }
        if (c.indexOf(name) == 0) {
            return c.substring(name.length, c.length);
        }
    }
    return "";
}

function setCookie(cname, cvalue, exdays) {
    const d = new Date();
    d.setTime(d.getTime() + (exdays*24*60*60*1000));
    let expires = "expires="+ d.toUTCString();
    document.cookie =  cname + "=" + cvalue + ";" + expires + ";path=/";
}