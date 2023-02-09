const oneMinute = 1000 * 60; // 一分钟的毫秒数
const oneHour = oneMinute * 60; // 一小时的毫秒数
const oneDay = oneHour * 24; // 一天的毫秒数
const oneWeek = oneDay * 7; // 一星期的毫秒数
const oneMonth = oneDay * 30; // 一个月的毫秒数

/**
 * 按天数减少
 *
 * @param days 要减少的天数
 */
Date.prototype.minusDays = function (days) {
    return this.minusMillis(oneDay * days);
};

/**
 * 按天数增加
 *
 * @param days 要增加的天数
 */
Date.prototype.plusDays = function (days) {
    return this.plusMillis(oneDay * days);
};

/**
 * 按小时减少
 *
 * @param hours 要减少的小时数
 */
Date.prototype.minusHours = function (hours) {
    return this.minusMillis(oneHour * hours);
};

/**
 * 按小时增加
 *
 * @param hours 要增加的小时数
 */
Date.prototype.plusHours = function (hours) {
    return this.plusMillis(oneHour * hours);
};

/**
 * 按分钟减少
 *
 * @param minutes 要减少的分钟数
 */
Date.prototype.minusMinutes = function (minutes) {
    return this.minusMillis(oneMinute * minutes);
};

/**
 * 按分钟增加
 *
 * @param minutes 要增加的分钟数
 */
Date.prototype.plusMinutes = function (minutes) {
    return this.plusMillis(oneMinute * minutes);
};

/**
 * 按毫秒减少
 *
 * @param millis 要减少的毫秒数
 */
Date.prototype.minusMillis = function(millis) {
    let time = this.getTime() - millis;
    let newDate = new Date();
    newDate.setTime(time);
    return newDate;
};

/**
 * 按毫秒增加
 *
 * @param millis 要增加的毫秒数
 */
Date.prototype.plusMillis = function(millis) {
    let time = this.getTime() + millis;
    let newDate = new Date();
    newDate.setTime(time);
    return newDate;
};

/**
 * 设置时间为当天的 00:00:00.000
 */
Date.prototype.setMinTime = function () {
    this.setHours(0);
    this.setMinutes(0);
    this.setSeconds(0);
    this.setMilliseconds(0);
    return this;
};

/**
 * 设置时间为当天的 23:59:59.999
 */
Date.prototype.setMaxTime = function () {
    this.setHours(23);
    this.setMinutes(59);
    this.setSeconds(59);
    this.setMilliseconds(999);
    return this;
};

/**
 * 格式化日期
 */
Date.prototype.formatDate = function () {
    return this.getFullYear() + "-" + addZero(this.getMonth() + 1) + "-" + addZero(this.getDate());
};

/**
 * 格式化时间
 */
Date.prototype.formatTime = function () {
    return addZero(this.getHours()) + ":" + addZero(this.getMinutes()) + ":" + addZero(this.getSeconds());
};

/**
 * 格式化日期加时间
 *
 * @param split 日期和时间之间的分隔符，默认是一个空格
 */
Date.prototype.formatDateTime = function (split = ' ') {
    return this.formatDate() + split + this.formatTime();
};

class DateUtil {

    // 字符串转 Date 对象
    static parseDate(str) {
        return new Date(str.replace(/-/g, '/'));
    }

    static formatMillis(millis) {
        return moment(millis).format('YYYY-M-D H:m:s')
    }

    static firstDayOfMonth() {
        const date = new Date();
        date.setDate(1);
        date.setMinTime();
        return date;
    }
}