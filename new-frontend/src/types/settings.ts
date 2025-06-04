// Based on web/entity/entity.go AllSetting struct
export interface AllSetting {
  webListen?: string;
  webDomain?: string;
  webPort?: number;
  webCertFile?: string;
  webKeyFile?: string;
  webBasePath?: string;
  sessionMaxAge?: number;
  pageSize?: number;
  expireDiff?: number;
  trafficDiff?: number;
  remarkModel?: string;
  tgBotEnable?: boolean;
  tgBotToken?: string;
  tgBotProxy?: string;
  tgBotAPIServer?: string;
  tgBotChatId?: string;
  tgRunTime?: string;
  tgBotBackup?: boolean;
  tgBotLoginNotify?: boolean;
  tgCpu?: number;
  tgLang?: string;
  timeLocation?: string;
  twoFactorEnable?: boolean;
  twoFactorToken?: string;
  subEnable?: boolean;
  subTitle?: string;
  subListen?: string;
  subPort?: number;
  subPath?: string;
  subDomain?: string;
  subCertFile?: string;
  subKeyFile?: string;
  subUpdates?: number;
  externalTrafficInformEnable?: boolean;
  externalTrafficInformURI?: string;
  subEncrypt?: boolean;
  subShowInfo?: boolean;
  subURI?: string;
  subJsonPath?: string;
  subJsonURI?: string;
  subJsonFragment?: string;
  subJsonNoises?: string;
  subJsonMux?: string;
  subJsonRules?: string;
  datepicker?: string;
}

// For updating username/password
export interface UpdateUserPayload {
    oldUsername?: string;
    oldPassword?: string;
    newUsername?: string;
    newPassword?: string;
}
