import datePickerZhCN from "antd/es/date-picker/locale/zh_CN";
import zhCN from "antd/es/locale/zh_CN";

const chineseDatePickerLocale = {
  ...datePickerZhCN,
  lang: {
    ...datePickerZhCN.lang,
    monthBeforeYear: false,
    yearFormat: "YYYY年",
    monthFormat: "M月",
    shortWeekDays: ["日", "一", "二", "三", "四", "五", "六"],
    shortMonths: ["1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"],
  },
} as typeof datePickerZhCN;

export const appDatePickerLocale = chineseDatePickerLocale;

export const appAntdLocale = {
  ...zhCN,
  DatePicker: chineseDatePickerLocale,
} as typeof zhCN;
