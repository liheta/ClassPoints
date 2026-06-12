import { strToU8, zipSync } from "fflate";

const XML_HEADER = '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>';

export function createXlsxBlob(sheetName, rows) {
  const safeSheetName = sanitizeSheetName(sheetName);
  const worksheet = buildWorksheet(rows);
  const now = new Date().toISOString();
  const files = {
    "[Content_Types].xml": xml(`
      <Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
        <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
        <Default Extension="xml" ContentType="application/xml"/>
        <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
        <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
        <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
        <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
        <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
      </Types>`),
    "_rels/.rels": xml(`
      <Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
        <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
        <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
        <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
      </Relationships>`),
    "docProps/core.xml": xml(`
      <cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
        xmlns:dc="http://purl.org/dc/elements/1.1/"
        xmlns:dcterms="http://purl.org/dc/terms/"
        xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
        <dc:creator>班级积分系统</dc:creator>
        <cp:lastModifiedBy>班级积分系统</cp:lastModifiedBy>
        <dcterms:created xsi:type="dcterms:W3CDTF">${now}</dcterms:created>
        <dcterms:modified xsi:type="dcterms:W3CDTF">${now}</dcterms:modified>
      </cp:coreProperties>`),
    "docProps/app.xml": xml(`
      <Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"
        xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
        <Application>班级积分系统</Application>
      </Properties>`),
    "xl/workbook.xml": xml(`
      <workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
        xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
        <sheets><sheet name="${escapeXml(safeSheetName)}" sheetId="1" r:id="rId1"/></sheets>
      </workbook>`),
    "xl/_rels/workbook.xml.rels": xml(`
      <Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
        <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
        <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
      </Relationships>`),
    "xl/styles.xml": xml(`
      <styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
        <fonts count="2">
          <font><sz val="11"/><name val="Microsoft YaHei"/><family val="2"/></font>
          <font><b/><sz val="11"/><color rgb="FF4F4A79"/><name val="Microsoft YaHei"/><family val="2"/></font>
        </fonts>
        <fills count="3">
          <fill><patternFill patternType="none"/></fill>
          <fill><patternFill patternType="gray125"/></fill>
          <fill><patternFill patternType="solid"><fgColor rgb="FFEDE9FE"/><bgColor indexed="64"/></patternFill></fill>
        </fills>
        <borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
        <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
        <cellXfs count="2">
          <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyAlignment="1"><alignment vertical="center"/></xf>
          <xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
        </cellXfs>
        <cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>
      </styleSheet>`),
    "xl/worksheets/sheet1.xml": worksheet,
  };

  const archive = zipSync(
    Object.fromEntries(Object.entries(files).map(([path, content]) => [path, strToU8(content)])),
    { level: 6 },
  );
  return new Blob([archive], {
    type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  });
}

function buildWorksheet(rows) {
  const rowCount = Math.max(rows.length, 1);
  const columnCount = Math.max(...rows.map((row) => row.length), 1);
  const range = `A1:${columnName(columnCount)}${rowCount}`;
  const sheetRows = rows
    .map((row, rowIndex) => {
      const cells = row.map((value, columnIndex) => buildCell(value, rowIndex, columnIndex)).join("");
      return `<row r="${rowIndex + 1}">${cells}</row>`;
    })
    .join("");
  const widths = [8, 16, 10, 18, 10, 24, 28, 12, 24, 22];
  const columns = widths.map((width, index) => `<col min="${index + 1}" max="${index + 1}" width="${width}" customWidth="1"/>`).join("");

  return xml(`
    <worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
      <dimension ref="${range}"/>
      <sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>
      <sheetFormatPr defaultRowHeight="20"/>
      <cols>${columns}</cols>
      <sheetData>${sheetRows}</sheetData>
      <autoFilter ref="${range}"/>
    </worksheet>`);
}

function buildCell(value, rowIndex, columnIndex) {
  const reference = `${columnName(columnIndex + 1)}${rowIndex + 1}`;
  const style = rowIndex === 0 ? ' s="1"' : "";
  if (typeof value === "number" && Number.isFinite(value)) {
    return `<c r="${reference}"${style}><v>${value}</v></c>`;
  }
  const text = cleanXmlText(value);
  const preserveSpace = /^\s|\s$/.test(text) ? ' xml:space="preserve"' : "";
  return `<c r="${reference}" t="inlineStr"${style}><is><t${preserveSpace}>${escapeXml(text)}</t></is></c>`;
}

function columnName(index) {
  let value = index;
  let result = "";
  while (value > 0) {
    value -= 1;
    result = String.fromCharCode(65 + (value % 26)) + result;
    value = Math.floor(value / 26);
  }
  return result;
}

function sanitizeSheetName(value) {
  const name = String(value || "积分记录").replace(/[\\/*?:[\]]/g, "-").trim();
  return (name || "积分记录").slice(0, 31);
}

function cleanXmlText(value) {
  return String(value ?? "").replace(/[\u0000-\u0008\u000B\u000C\u000E-\u001F]/g, "");
}

function escapeXml(value) {
  return cleanXmlText(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
}

function xml(content) {
  return `${XML_HEADER}${content.replace(/^\s+/gm, "").trim()}`;
}
