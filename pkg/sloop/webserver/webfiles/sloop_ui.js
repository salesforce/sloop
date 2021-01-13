/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

const palette = {
        baseDark: ["#2E3440", "#3B4252", "#434C5E", "#4C566A"],
        baseLight: ["#D8DEE9", "#E5E9F0", "#ECEFF4", "#F3F3FE"],
        primary: ["#8FBCBB", "#88C0D0", "#81A1C1", "#5E81AC"],
        highlight: ["#BF616A", "#D08770", "#EBCB8B", "#A3BE8C",
            "#B48EAD","#ADA8B6","#da7650","#D496A7","#ABC4AB",
            "#A4A6D2","#91BEF2","#97B1A6","#8A9B68"],
        severity: ['#4BD855', '#E0E000', '#D84B4B'],
};


// Globals Live Here
let topAxis, bottomAxis;

// These funcs get called whenever the propertes of either axis are changed
let topAxisDrawFunc, bottomAxisDrawFunc;

// These are d3 structs
let xAxisScale, yAxisBand;

// Array containing data retreived from sloop server
let data;

// The time displayed on certain mouseover and mousemove events
let theTime;

// These define the maximum drawing space on the window. I don't think
// this is the correct way of using these vars - it doesn't really respect window resizing
// and weird minimums and display scaling could potentially cause problems.
let displayMaxX, displayMaxY;

// Vertical spacing between bars
const resourceBarVerticalSpacing = 0.2;

// Since we're drawing bars on bars within the same yAxisBandwidth -
// This margin defines the space between the resource bar - and it's containing
// band within in the yAxisBand
let smallBarMargin;

let margin = {
    top: 20,
    left: 100
};

window.onresize = initializeDimensions;

function initializeDimensions() {
    displayMaxX = document.documentElement.clientWidth;
    displayMaxY = document.documentElement.clientHeight;
}

detailedToolTipIsVisible = false;

let noSortFn = function () {
    return 0
};

const compareStartFn = function (a, b) {
    if (a.kind != b.kind) {
        return compareKind(a, b)
    }
    return a.start - b.start;
};
const compareMostEventsFn = function (a, b) {
    if (a.kind != b.kind) {
        return compareKind(a, b)
    }
    return b.overlays.length - a.overlays.length;
};
const compareNameFn = function (a, b) {
    if (a.kind != b.kind) {
        return compareKind(a, b)
    }
    return ('' + a.text).localeCompare(b.text);
};
let cmpFn = noSortFn;

payload = d3.json(dataQueryUrl);
payload.then(function (result) {
    initializeDimensions();
    let svg = render(result);
    bindMouseEvents(svg);
    appendAxes(svg);
    renderTooltip();
});

function render(result) {
    let data = processAndSortResources(result);
    let dataByKind, kinds, filteredData;


    if (!data) {
        xAxisScale = d3.scaleUtc().range([margin.left, displayMaxX - margin.left]);
        yAxisBand = d3.scaleBand().padding(resourceBarVerticalSpacing);

        topAxisDrawFunc = d3.axisTop(xAxisScale);
        bottomAxisDrawFunc = d3.axisBottom(xAxisScale);
        filteredData = []
    } else {
        dataByKind = d3.nest().key(d => d.kind).entries(data);
        kinds = dataByKind.map(d => d.key);

        barColorGenFunc = d3.scaleOrdinal().domain(kinds).range(palette.highlight);
        severityColorGenFunc = d3.scaleLinear().domain([0, 1, 2]).range(palette.severity);

        xAxisScale = d3.scaleUtc()
            .domain([d3.min(data, d => d.start), d3.max(data, d => d.end)])
            .range([margin.left, displayMaxX - margin.left]);

        yAxisBand = d3.scaleBand()
            .domain(d3.range(data.length))
            .range([margin.top, (data.length * (30)) - margin.top])
            .padding(resourceBarVerticalSpacing);

        smallBarMargin = 0.1 * yAxisBand.bandwidth();


        filteredData = [].concat.apply([], dataByKind.map(d => d.values));
        filteredData.forEach(d => d.color = d3.color(barColorGenFunc(d.kind)));
    }

    topAxisDrawFunc = d3.axisTop(xAxisScale);
    bottomAxisDrawFunc = d3.axisBottom(xAxisScale);

    let svgWidth = xAxisScale.range()[1] + (2 * margin.left);
    let svgHeight = yAxisBand.range()[1] + (2 * margin.top);

    let svg = d3.select("#d3_here")
        .append("svg")
        .attr("viewBox", `0 0 ${svgWidth} ${svgHeight}`)
        .classed("svg-content", true);



    g = svg.append("g");
    // Create the graphical representation of each resource
    groups = g
        .selectAll("g")
        .data(filteredData)
        .enter()
        .append("g")
        .attr("transform", (d, i) => `translate(0 ${yAxisBand(i) + smallBarMargin})`)
        .each(createResourceBar);

    document.querySelector("body").groups = groups;
    return svg
}

severity = new Map([["Normal",0],["Warning",1],["Error",2]]);

function processAndSortResources(result) {
    let viewOptions = result.view_options;

    if (!result.rows) {
        data = {}
    } else {
        data = result.rows.map(d => {
            cmpFn = compareStartFn;
            switch (viewOptions.sort) {
                case "starttime":
                    cmpFn = compareStartFn;
                    break;
                case "name":
                    cmpFn = compareNameFn;
                    break;
                case "mostevents":
                    cmpFn = compareMostEventsFn;
                    break;
                default:
                    console.log("Unknown sort: " + viewOptions.sort);
                    break;
            }

            result = {
                ...d,
                start: d.start_date * 1000,
                end: (d.start_date * 1000) + (d.duration * 1000),
                overlays: d.overlays.map(e => {
                    // e is the Overlay struct defined in
                    // pkg/sloop/queries/types.go
                    let splitText = e.text.split(" ")
                    let worstSeverity = d3.max(splitText, text => {
                        return severity.get(text.split(":")[1])
                    });

                    let overlay = {
                        ...e,
                        start: (e.start_date * 1000),
                        end: (e.start_date * 1000) + (e.duration * 1000),
                        severity: worstSeverity,
                        reason: e.text,
                        count: splitText[2],
                    };
                    return overlay
                })
            };
            return result
        }).sort(cmpFn);
        return data
    }
}

function compareKind(a, b) {
    return ('' + a.kind).localeCompare(b.kind)
}

function appendAxes(svg) {
    line = svg.append("line")
        .attr("y1", yAxisBand.range()[0])
        .attr("y2", yAxisBand.range()[1])
        .attr("stroke", "rgba(0,0,0,0.5)")
        .style("pointer-events", "none");

    topAxis = svg
        .append("g")
        .attr("transform", () => `translate(0 ${yAxisBand.range()[0]})`)
        .call(topAxisDrawFunc)
        .attr("stroke", palette.baseLight[1])
        .classed("topAxis", true);

    bottomAxis = svg
        .append("g")
        .attr("transform", () => `translate(0 ${yAxisBand.range()[1]})`)
        .call(bottomAxisDrawFunc)
        .attr("stroke", palette.baseLight[1])
        .classed("bottomAxis", true);

}

function renderTooltip() {
    tooltip = d3.select("body")
        .append("div")
        .classed("tooltip", true)
}

function bindMouseEvents(svg) {
    svg.on("mousemove", function () {
        let [x, y] = d3.mouse(this);

        if (xAxisScale.invert(x) < xAxisScale.domain()[0] || (xAxisScale.invert(x) > xAxisScale.domain()[1])) {
            console.log("Vertical bar out of bounds x")
        } else if (y < yAxisBand.range()[0] || (y > yAxisBand.range()[1])) {
            console.log("Vertical bar out of bounds top")
        } else {
            line.attr("transform", `translate(${x} 0)`);
            theTime = xAxisScale.invert(x);
            if (!detailedToolTipIsVisible) {
                let tooltipX = d3.event.pageX;
                let tooltipY = d3.event.pageY;
                positionTooltip(tooltipX, tooltipY);
            }
        }
    });


    g.selectAll(".resource").on("mouseover", function (d) {
        if (!detailedToolTipIsVisible) {
            d3.select(this).attr("fill", d.color.darker());
            tooltip.style("opacity", 1)
        }
    }).on("mouseleave", function (d) {
        if (!detailedToolTipIsVisible) {
            d3.select(this).attr("fill", d3.color(barColorGenFunc(d.kind)));
            tooltip.style("opacity", 0)
        }
    }).on("mousemove", function (d) {
        if (!detailedToolTipIsVisible) {
            tooltip.html(getResourceBarContent(
                {
                    title: d.text,
                    kind: d.kind,
                    namespace: d.namespace,
                    time: theTime
                }
            ))
        }
    }).on("click", function (d) {
        showDetailedTooltip(d, d3.event, this);
    });
    // Intuitively 'd' should be the 'heatmap' element - but for whatever reason
    // the event binds correctly but 'd' is the resource element. Not sure why - I think
    // d3 binds events strangely like that.
    g.selectAll(".heatmap").on("mouseover", function (d) {
        if (!detailedToolTipIsVisible) {
            let parentColor = d.color.darker();
            let overlayIndex = parseInt(this.getAttribute("index"));
            let thisOverlay = d.overlays[overlayIndex];

            d3.select(this.parentElement).select(".resource").attr("fill", parentColor);
            d3.select(this).attr("fill", d3.color(barColorGenFunc(thisOverlay.text)).darker());

            let content = {
                text: thisOverlay.text,
                kind: d.kind,
                namespace: d.namespace,
                title: d.text,
                start: thisOverlay.start,
                end: thisOverlay.end,
            };

            thisOverlay = d.overlays[overlayIndex];
            d3.select(this).attr("fill", d3.color(severityColorGenFunc(thisOverlay.severity)).darker());
            d.overlays[overlayIndex].title = this.getAttribute("title");
            tooltip
                .style("opacity", 1)
                .html(getHeatmapContent(content));
        }
    }).on("mouseleave", function (d) {
        if (!detailedToolTipIsVisible) {
            d3.select(this.parentElement).select(".resource").attr("fill", d.color);

            let overlayIndex = parseInt(this.getAttribute("index"));
            let thisOverlay = d.overlays[overlayIndex];
            d3.select(this).attr("fill", severityColorGenFunc(thisOverlay.severity));
            tooltip.style("opacity", 0)
        }
    }).on("click", function (d) {
        showDetailedTooltip(d, d3.event, this);
    });
}

function getHeatmapContent(d) {
    let allReasons = d.text.split(" ").reduce((r, l, i, a) => {
        let splitText = l.split(":");
        let severityText = splitText[1];
        let severityCode = severity.get(splitText[1]);
        let severityColor = palette.severity[severityCode];
        return `<tr>
                 <td> <b> ${splitText[0]} </b> </td>
                 <td> <b> ${splitText[2]} </b> </td> 
                 <td> <b style="color:${severityColor}">${severityText}</b> </td>
                 </tr>` + r
    }, "");

    let table = `<table> <tr> <td>Reason</td> <td>Times Seen</td> <td>Severity</td> </tr> ${allReasons} </table>`;
    return `Name: <b>${d.title}</b><br/>
        Kind: <b>${d.kind}</b><br/>
        Namespace: <b>${d.namespace}</b><br />
        ${table}
        ${formatDateTime(d.start)} - ${formatDateTime(d.end)}`
}

function getResourceBarContent(d) {
    return `<div id="tiny-tooltip">Name: <b>${d.title}</b><br/>` +
        `Kind: <b>${d.kind}</b><br/>` +
        `Namespace: <b>${d.namespace}</b><br/>` +
        `<br/>${formatDateTime(d.time)}</div>`;
}

function formatDateTime(d) {
    return new Date(d).toUTCString()
}

function createResourceBar(d) {
    const el = d3.select(this);
    const sx = xAxisScale(d.start);

    let w = Math.max(xAxisScale(d.end) - xAxisScale(d.start), 10);
    const isLabelRight = (sx > displayMaxX / 2 ? sx + w < displayMaxX : sx - w > 0);

    el
        .append("rect")
        .attr("x", sx)
        .attr("height", yAxisBand.bandwidth() - (2 * smallBarMargin))
        .attr("width", w)
        .attr("fill", barColorGenFunc(d.kind))
        .classed("resource", true);

    let n = 0;

    // Print overlay heatmap for each object
    d.overlays.forEach(function (overlay) {
        const overlaySX = xAxisScale(overlay.start);
        const overlayW = xAxisScale(overlay.end) - xAxisScale(overlay.start);

        if ((overlaySX < sx) || ((overlaySX + overlayW) > (sx + w))) {
            n++;
            console.log("Overlay out of bounds for resource");
        } else {
            let text = "";
            if (d.text) {
                text = d.text
            }

            el
                .append("rect")
                .attr("x", overlaySX)
                .attr("y", yAxisBand.bandwidth() * 0.15)
                .attr("rx", 6)
                .attr("ry", 6)
                .attr("height", yAxisBand.bandwidth() * 0.6)
                .attr("width", overlayW * 0.75)
                .attr("fill", d3.color(severityColorGenFunc(overlay.severity)))
                .attr("stroke", palette.baseDark[3])
                .attr("stroke-width", "1px")
                .attr("title", text)
                .attr("transform", `translate(0 ${-smallBarMargin})`)
                .attr("index", n++)
                .classed("heatmap", true)
        }
    });

    if (d.nochangeat != null) {
        d.nochangeat.forEach(function (timestamp) {
            // add black tick mark at bottom of band - 1/10 of band
            el
                .append("rect")
                .attr("x", xAxisScale(timestamp*1000))
                .attr("y", 9 * (yAxisBand.bandwidth() / 10))
                .attr("height", yAxisBand.bandwidth() / 10)
                .attr("width", 1)
                .attr("fill", "black")
        });
    }

    if (d.changedat != null) {
        d.changedat.forEach(function (timestamp) {
            // add red tick mark at top of band - 1/5 of band
            el
                .append("rect")
                .attr("x", xAxisScale(timestamp*1000))
                .attr("height", yAxisBand.bandwidth() / 5)
                .attr("width", 1)
                .attr("fill", "red")
        });
    }

    el.append("text")
        .text(d.text)
        .attr("x", isLabelRight ? sx - 5 : sx + w + 5)
        .attr("fill", palette.baseLight[0])
        .classed("resource-bar-label", true)
        .style("text-anchor", isLabelRight ? "end" : "start");
}

function evalJSFromHtml(html) {
    let newElement = document.createElement('div');
    newElement.innerHTML = html;
    let scripts = newElement.getElementsByTagName("script");
    for (let i = 0; i < scripts.length; ++i) {
        eval(scripts[i].innerHTML);
    }
}

function positionTooltip(x, y) {
    let tooltipX = x;
    let tooltipY = y;

    if (x > displayMaxX / 2) {
        tooltip.style("right", (displayMaxX - tooltipX) + "px");
        tooltip.style("left", null)
    } else {
        tooltip.style("left", tooltipX + "px");
        tooltip.style("right", null)
    }

    if (y > displayMaxY / 2) {
        tooltip.style("bottom", (displayMaxY - tooltipY) + "px");
        tooltip.style("top", null)
    } else {
        // It looks really goofy if you don't. 20px is about the size of the mouse on a 1080 scaled display
        tooltip.style("top", tooltipY + 20 + "px");
        tooltip.style("bottom", null)
    }

    if (detailedToolTipIsVisible) {
        tooltip.classed("ignore-pointer-events", false)
    } else {
        tooltip.classed("ignore-pointer-events", true)
    }
}

function showDetailedTooltip(d, event, parent) {
    let tooltipX = event.pageX;
    let tooltipY = event.pageY;
    if (detailedToolTipIsVisible) {
        let resourceBarHtml = getResourceBarContent(
            {
                title: d.text,
                kind: d.kind,
                namespace: d.namespace,
                time: theTime
            }
        );
        tooltip.html(resourceBarHtml);
        positionTooltip(tooltipX, tooltipY);
        detailedToolTipIsVisible = false
    } else {
        let [x, y] = d3.mouse(parent);

        let tooltipX = event.pageX;
        let tooltipY = event.pageY;
        const resourceRequestPath = "/resource";
        $.ajax({
            url: resourceRequestPath,
            data: {
                click_time: xAxisScale.invert(x).getTime(),
                name: d.text,
                namespace: d.namespace,
                kind: d.kind,
            },
            success: function (result) {
                detailedToolTipIsVisible = true;
                tooltip.html(result);
                evalJSFromHtml(result);
                positionTooltip(tooltipX, tooltipY)
            }
        });
    }
}