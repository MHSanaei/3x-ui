/// <reference types="vite/client" />

interface SubPageData {
  sId?: string;
  enabled?: boolean;
  download?: string;
  upload?: string;
  total?: string;
  used?: string;
  remained?: string;
  totalByte?: string | number;
  expire?: string | number;
  lastOnline?: string | number;
  subUrl?: string;
  subJsonUrl?: string;
  subClashUrl?: string;
  subTitle?: string;
  links?: string[];
  datepicker?: 'gregorian' | 'jalalian';
  downloadByte?: string | number;
  uploadByte?: string | number;
  usedByte?: string | number;
}

interface Window {
  X_UI_BASE_PATH?: string;
  X_UI_CUR_VER?: string;
  __SUB_PAGE_DATA__?: SubPageData;
}
