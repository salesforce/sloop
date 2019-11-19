/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

const displayMaxX = document.documentElement.clientWidth;
const displayMaxY = document.documentElement.clientHeight;

let topAxis, bottomAxis, xAxisScale, yAxisBand, data, theTime, axisTop, axisBottom, smallBarOffset;

let margin = {
    top: 20,
    left: 100
};

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
    let svg = render(result);
    bindMouseEvents(svg);
    appendAxes(svg);
    renderTooltip();
});

function render(result) {
    let data = processAndSortResources(result);
    let dataByKind, kinds, filteredData;

    if (!data) {
        xAxisScale = d3.scaleUtc()
            .range([margin.left, displayMaxX - margin.left]);

        yAxisBand = d3.scaleBand()
            .range([margin.top, (50) - margin.top])
            .padding(0.2);


        axisTop = d3.axisTop(xAxisScale);
        axisBottom = d3.axisBottom(xAxisScale);
        filteredData = []
    } else {
        dataByKind = d3.nest().key(d => d.kind).entries(data);
        kinds = dataByKind.map(d => d.key);

        barColorGenFunc = d3.scaleOrdinal(d3.schemeSet2).domain(kinds);
        severityColorGenFunc = d3.scaleLinear().domain([0, 1, 2]).range(['#4BD855', '#D8D14B', '#D84B4B']);
        eventKindColorGenFunc = d3.scaleOrdinal(d3.schemeSet3).domain([]);

        xAxisScale = d3.scaleUtc()
            .domain([d3.min(data, d => d.start), d3.max(data, d => d.end)])
            .range([margin.left, displayMaxX - margin.left]);

        yAxisBand = d3.scaleBand()
            .domain(d3.range(data.length))
            .range([margin.top, (data.length * (30)) - margin.top])
            .padding(0.2);

        smallBarOffset = 0.1 * yAxisBand.bandwidth();


        filteredData = [].concat.apply([], dataByKind.map(d => d.values));
        filteredData.forEach(d => d.color = d3.color(barColorGenFunc(d.kind)));
    }

    axisTop = d3.axisTop(xAxisScale);
    axisBottom = d3.axisBottom(xAxisScale);

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
        .attr("transform", (d, i) => `translate(0 ${yAxisBand(i) + smallBarOffset})`)
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

                    console.log(JSON.stringify(overlay));
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
        .attr("stroke", "rgba(0,0,0,0.2)")
        .style("pointer-events", "none");

    topAxis = svg
        .append("g")
        .attr("transform", () => `translate(0 ${yAxisBand.range()[0]})`)
        .call(axisTop)
        .classed("topAxis", true);

    bottomAxis = svg
        .append("g")
        .attr("transform", () => `translate(0 ${yAxisBand.range()[1]})`)
        .call(axisBottom)
        .classed("bottomAxis", true);

}

function renderTooltip() {
    tooltip = d3.select("#tooltip_container")
        .append("div")
        .call(createTooltip);
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
        let splitText = l.split(":")
        return `<tr>
                 <td> <b style=color:${eventKindColorGenFunc(splitText[0])}>${splitText[0]}</b> </td>
                 <td> <b> ${splitText[2]} </b> </td> 
                 <td> <b style="color:${severityColorGenFunc(severity.get(splitText[1]))}">${splitText[1]}</b> </td>
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
    return '<div style="padding:10px">' +
        `Name: <b>${d.title}</b><br/>` +
        `Kind: <b>${d.kind}</b><br/>` +
        `Namespace: <b>${d.namespace}</b><br/>` +
        `<br/>${formatDateTime(d.time)}` +
        '</div>';
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
        .attr("height", yAxisBand.bandwidth() - (2 * smallBarOffset))
        .attr("width", w)
        .attr("fill", barColorGenFunc(d.kind))
        .style("cursor", "pointer")
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
                .attr("height", yAxisBand.bandwidth())
                .attr("width", overlayW)
                .attr("fill", d3.color(severityColorGenFunc(overlay.severity)))
                .attr("title", text)
                .attr("transform", `translate(0 ${-smallBarOffset})`)
                .style("cursor", "pointer")
                .classed("heatmap", true)
                .attr("index", n++)
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
        .attr("fill", "black")
        .style("text-anchor", isLabelRight ? "end" : "start")
        .style("dominant-baseline", "hanging")
        .style("font-size", "14");
}

function evalJSFromHtml(html) {
    let newElement = document.createElement('div');
    newElement.innerHTML = html;
    let scripts = newElement.getElementsByTagName("script");
    for (let i = 0; i < scripts.length; ++i) {
        eval(scripts[i].innerHTML);
    }
}

// I think the detailed tooltip should probably be moved to
// a modal dialog - it's getting to be too large and unwieldy
function createTooltip(el) {
    el
        .style("position", "absolute")
        .style("top", 0)
        .style("opacity", 0)
        .style("background", "white")
        .style("border-radius", "5px")
        .style("box-shadow", "0 0 10px rgba(0,0,0,.25)")
        .style("line-height", "1.3")
        .style("z-index", 1)
        .style("font", "12px sans-serif")
        .style("max-height", "50%")
        .style("max-width", "50%")
        .style("overflow-y", "scroll")
}

function positionTooltip(x, y) {
    let tooltipX = x;
    let tooltipY = y;

    if (x > displayMaxX / 2) {
        tooltip.style("right", (displayMaxX - tooltipX) + "px");
        tooltip.style("left", "")
    } else {
        tooltip.style("left", tooltipX + "px");
        tooltip.style("right", "")
    }

    if (y > displayMaxY / 2) {
        tooltip.style("bottom", (displayMaxY - tooltipY) + "px");
        tooltip.style("top", "")
    } else {
        // It looks really goofy if you don't. 20px is about the size of the mouse on a 1080 scaled display
        tooltip.style("top", tooltipY + "px");
        tooltip.style("bottom", "")
    }

    if (detailedToolTipIsVisible) {
        tooltip.style("pointer-events", "auto")
    } else {
        tooltip.style("pointer-events", "none")
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

        $.ajax({
            url: "/resource",
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