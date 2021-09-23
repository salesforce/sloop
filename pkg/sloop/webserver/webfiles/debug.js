/*
 * Copyright (c) 2021, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

// Sets the Home link reference for all the debug pages 
function loadHomeRef() {
    var homeRef = "/" + window.location.pathname.split('/')[1]; 
    document.getElementById("homeLink").setAttribute("href", homeRef);
    return;
}  
