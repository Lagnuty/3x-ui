const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

for (const filename of ['inbound.js', 'outbound.js']) {
    test(`${filename} preserves XDRIVE H3-era template and XMUX fields`, () => {
        const source = fs.readFileSync(path.join(__dirname, filename), 'utf8');
        const context = vm.createContext({});
        vm.runInContext(source, context, { filename });
        const settings = vm.runInContext(`new XDriveStreamSettings({
            service: 'template', remoteFolder: 'test',
            template: { put: { url: 'https://api.example/upload', hrefPath: 'href', hrefMethod: 'PUT' } },
            xmux: { maxConnections: 4, hMaxReusableSecs: '1800-3000' }
        })`, context);
        assert.equal(settings.enableXmux, true);
        assert.equal(settings.toJson().template.put.hrefPath, 'href');
        assert.equal(settings.toJson().template.put.hrefMethod, 'PUT');
        assert.equal(settings.toJson().xmux.maxConnections, 4);
        settings.templateText = JSON.stringify({ get: { url: 'https://api.example/download', hrefPath: 'href' } });
        assert.equal(settings.toJson().template.get.hrefPath, 'href');
        settings.enableXmux = false;
        assert.equal(settings.toJson().xmux, undefined);
    });
}
