import { ObjectUtil } from '@/utils';

export class AllSetting {
  webListen = '';
  webDomain = '';
  webPort = 2053;
  webCertFile = '';
  webKeyFile = '';
  webBasePath = '/';
  sessionMaxAge = 360;
  trustedProxyCIDRs = '127.0.0.1/32,::1/128';
  pageSize = 25;
  expireDiff = 0;
  trafficDiff = 0;
  remarkModel = '-io';
  datepicker: 'gregorian' | 'jalalian' = 'gregorian';
  tgBotEnable = false;
  tgBotToken = '';
  tgBotProxy = '';
  tgBotAPIServer = '';
  tgBotChatId = '';
  tgRunTime = '@daily';
  tgBotBackup = false;
  tgBotLoginNotify = true;
  tgCpu = 80;
  tgLang = 'en-US';
  twoFactorEnable = false;
  twoFactorToken = '';
  xrayTemplateConfig = '';
  subEnable = true;
  subJsonEnable = false;
  subTitle = '';
  subSupportUrl = '';
  subProfileUrl = '';
  subAnnounce = '';
  subEnableRouting = true;
  subRoutingRules = '';
  subListen = '';
  subPort = 2096;
  subPath = '/sub/';
  subJsonPath = '/json/';
  subClashEnable = true;
  subClashPath = '/clash/';
  subDomain = '';
  externalTrafficInformEnable = false;
  externalTrafficInformURI = '';
  restartXrayOnClientDisable = true;
  subCertFile = '';
  subKeyFile = '';
  subUpdates = 12;
  subEncrypt = true;
  subShowInfo = true;
  subEmailInRemark = true;
  subURI = '';
  subJsonURI = '';
  subClashURI = '';
  subJsonFragment = '';
  subJsonNoises = '';
  subJsonMux = '';
  subJsonRules = '';

  timeLocation = 'Local';

  ldapEnable = false;
  ldapHost = '';
  ldapPort = 389;
  ldapUseTLS = false;
  ldapBindDN = '';
  ldapPassword = '';
  ldapBaseDN = '';
  ldapUserFilter = '(objectClass=person)';
  ldapUserAttr = 'mail';
  ldapVlessField = 'vless_enabled';
  ldapSyncCron = '@every 1m';
  ldapFlagField = '';
  ldapTruthyValues = 'true,1,yes,on';
  ldapInvertFlag = false;
  ldapInboundTags = '';
  ldapAutoCreate = false;
  ldapAutoDelete = false;
  ldapDefaultTotalGB = 0;
  ldapDefaultExpiryDays = 0;
  ldapDefaultLimitIP = 0;
  hasTgBotToken = false;
  hasTwoFactorToken = false;
  hasLdapPassword = false;
  hasApiToken = false;
  hasWarpSecret = false;
  hasNordSecret = false;

  constructor(data?: unknown) {
    if (data != null) {
      ObjectUtil.cloneProps(this, data);
    }
  }

  equals(other: AllSetting): boolean {
    return ObjectUtil.equals(this, other);
  }
}
