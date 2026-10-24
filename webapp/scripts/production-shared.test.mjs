// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable no-process-env */

import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {mkdtemp, mkdir, writeFile, rm} from 'node:fs/promises';
import {createRequire} from 'node:module';
import {tmpdir} from 'node:os';
import path from 'node:path';
import {test} from 'node:test';
import {fileURLToPath, pathToFileURL} from 'node:url';

import babel from '@babel/core';
import webpack from 'webpack';
import {rollup} from 'rollup';
import loadConfigFile from 'rollup/dist/loadConfigFile.js';

const require = createRequire(import.meta.url);
const webapp = fileURLToPath(new URL('../', import.meta.url));
const channels = path.join(webapp, 'channels');

test('components library leaves shared components and React runtime imports external', async () => {
    const directory = await mkdtemp(path.join(webapp, 'node_modules/.production-components-'));
    try {
        const input = path.join(directory, 'entry.js');
        await writeFile(input, [
            "export {Button} from '@mattermost/shared/components/button';",
            "export {jsx} from 'react/jsx-runtime';",
            "export {jsxDEV} from 'react/jsx-dev-runtime';",
        ].join('\n'));
        const {options} = await loadConfigFile(path.join(webapp, 'platform/components/rollup.config.js'));
        const config = options[0];
        // The fixture is already JS and has no styles; keep the real dependency resolvers.
        const bundle = await rollup({
            ...config,
            input,
            plugins: config.plugins.filter((plugin) => !['typescript', 'scss'].includes(plugin.name)),
        });
        try {
            const {output} = await bundle.generate({format: 'es'});
            const code = output[0].code;
            assert.match(code, /from ['"]@mattermost\/shared\/components\/button['"]/);
            assert.match(code, /from ['"]react\/jsx-runtime['"]/);
            assert.match(code, /from ['"]react\/jsx-dev-runtime['"]/);
            assert.doesNotMatch(code, /react-jsx-dev-runtime\.(?:production|development)/);
        } finally {
            await bundle.close();
        }
    } finally {
        await rm(directory, {recursive: true, force: true});
    }
});

test('production renders SharedProvider and Button even when shared dist contains development JSX', async () => {
    const directory = await mkdtemp(path.join(tmpdir(), 'mattermost-production-shared-'));
    try {
        // Reproduce Parcel watch output without changing the developer's real dist.
        const shared = path.join(directory, 'node_modules/@mattermost/shared');
        await mkdir(shared, {recursive: true});
        await writeFile(path.join(shared, 'package.json'), JSON.stringify({
            name: '@mattermost/shared',
            exports: {'./context': './context.js'},
        }));
        const development = babel.transformFileSync(path.join(webapp, 'platform/shared/src/context/context.tsx'), {
            configFile: path.join(channels, 'babel.config.js'),
            presets: [['@babel/preset-react', {runtime: 'automatic', development: true}]],
        });
        assert.match(development.code, /jsxDEV/);
        await writeFile(path.join(shared, 'context.js'), development.code);
        const entry = path.join(directory, 'app.js');
        await writeFile(entry, [
            "export {SharedProvider} from '@mattermost/shared/context';",
            "export {Button} from '@mattermost/shared/components/button';",
        ].join('\n'));

        // Use the app's real production resolver and JavaScript loaders.
        const previousTarget = process.env.npm_lifecycle_event;
        process.env.npm_lifecycle_event = 'build';
        let production;
        try {
            const {default: createProductionConfig} = await import(pathToFileURL(path.join(channels, 'webpack.config.js')).href);
            production = await createProductionConfig();
        } finally {
            if (previousTarget === undefined) {
                delete process.env.npm_lifecycle_event;
            } else {
                process.env.npm_lifecycle_event = previousTarget;
            }
        }
        const config = {
            mode: 'production',
            context: channels,
            target: 'node',
            entry,
            output: {path: directory, filename: 'provider.cjs', library: {type: 'commonjs2'}},
            resolve: {...production.resolve, modules: [path.join(directory, 'node_modules'), path.join(webapp, 'node_modules')]},
            module: {rules: production.module.rules.filter((rule) => rule.use?.loader === 'babel-loader')},
            externals: Object.fromEntries(['react', 'react/jsx-runtime', 'react/jsx-dev-runtime'].map((name) => [name, `commonjs ${require.resolve(name)}`])),
            optimization: {minimize: false},
        };
        const compiler = webpack(config);
        try {
            const stats = await new Promise((resolve, reject) => compiler.run((err, result) => (err ? reject(err) : resolve(result))));
            assert.equal(stats.hasErrors(), false, stats.toString({all: false, errors: true}));
        } finally {
            await new Promise((resolve, reject) => compiler.close((err) => (err ? reject(err) : resolve())));
        }
        const output = execFileSync(process.execPath, ['-e', `
            global.window = {};
            const React = require(${JSON.stringify(require.resolve('react'))});
            const {renderToStaticMarkup} = require(${JSON.stringify(require.resolve('react-dom/server'))});
            const {SharedProvider, Button} = require(${JSON.stringify(path.join(directory, 'provider.cjs'))});
            process.stdout.write(renderToStaticMarkup(React.createElement(SharedProvider, {
                useEmojiByName: () => undefined,
                useEmojiUrl: () => '',
            }, React.createElement(Button, null, 'Production button works'))));
        `], {env: {...process.env, NODE_ENV: 'production'}, encoding: 'utf8'});
        assert.match(output, /^<button[^>]*>Production button works<\/button>$/);
    } finally {
        await rm(directory, {recursive: true, force: true});
    }
});
