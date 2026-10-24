// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// create_mermaid_worker uses import.meta.url so webpack can bundle the worker, which Jest's CommonJS
// runtime cannot parse. Tests that render diagrams set the returned worker with mockReturnValue.
export default jest.fn();
