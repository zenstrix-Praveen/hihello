const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const root = path.resolve(__dirname, '../src');
function files(folder) { return fs.readdirSync(folder,{withFileTypes:true}).flatMap(entry=>entry.isDirectory()?files(path.join(folder,entry.name)):[path.join(folder,entry.name)]); }
const targets = [
  ...files(path.join(root,'app/dashboard')), ...files(path.join(root,'components/dashboard')),
  ...['login','register','signin','forgot-password','reset-password','verify-otp'].flatMap(route=>files(path.join(root,'app',route))),
  'components/auth/CustomerOTPForm.tsx','components/cards/CardPreview.tsx','components/templates/TemplatePreview.tsx','components/public/Chatbot.tsx',
].map(file=>path.isAbsolute(file)?file:path.join(root,file)).filter(file=>file.endsWith('.tsx'));
const allowed = /^(?:QR|NFC|OTP|ID|URL|Dbizcard|https?:\/\/.*|[\w.+-]+@[\w.-]+\.[a-z]+)$/i;
const findings = [];
for (const file of targets) {
  const source=ts.createSourceFile(file,fs.readFileSync(file,'utf8'),ts.ScriptTarget.Latest,true,ts.ScriptKind.TSX);
  function report(node,text,kind) {
    text=text.replace(/\s+/g,' ').trim();
    if (!/[A-Za-z]/.test(text)||allowed.test(text)) return;
    findings.push({file:path.relative(root,file),line:source.getLineAndCharacterOfPosition(node.getStart()).line+1,kind,text});
  }
  function visit(node) {
    if(ts.isJsxText(node)) report(node,node.text,'text');
    if(ts.isJsxAttribute(node) && ['placeholder','aria-label','title','alt'].includes(node.name.getText()) && node.initializer && ts.isStringLiteral(node.initializer)) report(node,node.initializer.text,node.name.getText());
    if(ts.isJsxExpression(node)&&node.expression&&ts.isStringLiteral(node.expression)) report(node,node.expression.text,'expression');
    ts.forEachChild(node,visit);
  }
  visit(source);
}
console.log(JSON.stringify({components:targets.length,findings},null,2));
process.exitCode = findings.length ? 1 : 0;
