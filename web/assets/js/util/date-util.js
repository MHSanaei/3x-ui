const oneMinute = 1000 * 60;  // MilliseConds in a Minute
const oneHour = oneMinute * 60;  // The milliseconds of one hour
const oneDay = oneHour * 24; // The Number of MilliseConds A Day
const oneWeek = oneDay * 7; // The milliseconds per week
const oneMonth = oneDay * 30; // The milliseconds of a month

/**
 * Decrease according to the number of days
 *
 * @param days to reduce the number of days to be reduced
 */
Date.prototype.minusDays = function (days) {
    return this.minusMillis(oneDay * days);
};

/**
 * Increase according to the number of days
 *
 * @param days The number of days to be increased
 */
Date.prototype.plusDays = function (days) {
    return this.plusMillis(oneDay * days);
};

/**
 * A few
 *
 * @param hours to be reduced
 */
Date.prototype.minusHours = function (hours) {
    return this.minusMillis(oneHour * hours);
};

/**
 * Increase hourly
 *
 * @param hours to increase the number of hours
 */
Date.prototype.plusHours = function (hours) {
    return this.plusMillis(oneHour * hours);
};

/**
 * Make reduction in minutes
 *
 * @param minutes to reduce the number of minutes
 */
Date.prototype.minusMinutes = function (minutes) {
    return this.minusMillis(oneMinute * minutes);
};

/**
 * Add in minutes
 *
 * @param minutes to increase the number of minutes
 */
Date.prototype.plusMinutes = function (minutes) {
    return this.plusMillis(oneMinute * minutes);
};

/**
 * Decrease in milliseconds
 *
 * @param millis to reduce the milliseconds
 */
Date.prototype.minusMillis = function(millis) {
    let time = this.getTime() - millis;
    let newDate = new Date();
    newDate.setTime(time);
    return newDate;
};

/**
 * Add in milliseconds to increase
 *
 * @param millis to increase the milliseconds to increase
 */
Date.prototype.plusMillis = function(millis) {
    let time = this.getTime() + millis;
    let newDate = new Date();
    newDate.setTime(time);
    return newDate;
};

/**
 * Setting time is 00: 00: 00.000 on the day
 */
Date.prototype.setMinTime = function () {
    this.setHours(0);
    this.setMinutes(0);
    this.setSeconds(0);
    this.setMilliseconds(0);
    return this;
};

/**
 * Setting time is 23: 59: 59.999 on the same day
 */
Date.prototype.setMaxTime = function () {
    this.setHours(23);
    this.setMinutes(59);
    this.setSeconds(59);
    this.setMilliseconds(999);
    return this;
};

/**
 * Formatting date
 */
Date.prototype.formatDate = function () {
    return this.getFullYear() + "-" + addZero(this.getMonth() + 1) + "-" + addZero(this.getDate());
};

/**
 * Format time
 */
Date.prototype.formatTime = function () {
    return addZero(this.getHours()) + ":" + addZero(this.getMinutes()) + ":" + addZero(this.getSeconds());
};

/**
 * Formatting date plus time
 *
 * @param split Date and time separation symbols, default is a space
 */
Date.prototype.formatDateTime = function (split = ' ') {
    return this.formatDate() + split + this.formatTime();
};

class DateUtil {
    // String to date object
    static parseDate(str) {
        return new Date(str.replace(/-/g, '/'));
    }

    static formatMillis(millis) {
        return moment(millis).format('YYYY-M-D H:m:s');
    }

    static firstDayOfMonth() {
        const date = new Date();
        date.setDate(1);
        date.setMinTime();
        return date;
    }

    static convertToJalalian(date) {
        return date && moment.isMoment(date) ? date.format('jYYYY/jMM/jDD HH:mm:ss') : null;
    }

}
