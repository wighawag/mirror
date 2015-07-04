var reload = function () {

 
var hop = Object.prototype.hasOwnProperty,
      head = document.getElementsByTagName("head")[0];
  
 /**
 * Fetch all the local stylesheets from the page.
 *
 * @returns {Object} The list of local stylesheets keyed by their base URL.
 */
function getLocalStylesheets() {

  /**
   * Checks if the stylesheet is local.
   *
   * @param {Object} link The link to check for.
   * @returns {Boolean}
   */
  function isLocalStylesheet(link) {
	var href, i, isExternal = true;
	if (link.getAttribute("rel") !== "stylesheet") {
	  return false;
	}
	href = link.href;

	for (i = 0; i < script.bases.length; i += 1) {
	  if (href.indexOf(script.bases[i]) > -1) {
		isExternal = false;
		break;
	  }
	}

	return !(isExternal && href.match(/^https?:/));
  }

  /**
   * Checks if the stylesheet's media attribute is 'print'
   *
   * @param (Object) link The stylesheet element to check.
   * @returns (Boolean)
   */
  function isPrintStylesheet(link) {
	return link.getAttribute("media") === "print";
  }

  /**
   * Get the link's base URL.
   *
   * @param {String} href The URL to check.
   * @returns {String|Boolean} The base URL, or false if no matches found.
   */
  function getBase(href) {
	var base, j;
	//TODO for (j = 0; j < script.bases.length; j += 1) {
	  base = document.location.protocol + "//" + document.location.host;//TODO script.bases[j];
	  if (href.indexOf(base) > -1) {
		return href.substr(base.length);
	  }
	//}
	return false;
  }

  function getProperty(property) {
	return this[property];
  }

  var stylesheets = {},
	  reImport = /@import\s+url\(["']?([^"'\)]+)["']?\)/g,
	  links = document.getElementsByTagName("link"),
	  link, href, matches, content, i, m;

  // Go through all the links in the page, looking for stylesheets.
  for (i = 0, m = links.length; i < m; i += 1) {
	link = links[i];
	if (isPrintStylesheet(link)) continue;
	//TODO if (!isLocalStylesheet(link)) continue;
	// Link is local, get the base URL.
	href = getBase(link.href);
	if (href !== false) {
	  stylesheets[href] = link;
	}
  }

  // Go through all the style tags, looking for @import tags.
  links = document.getElementsByTagName("style");
  for (i = 0, m = links.length; i < m; i += 1) {
	if (isPrintStylesheet(links[i])) continue;
	content = links[i].text || links[i].textContent;
	while ((matches = reImport.exec(content))) {
	  link = {
		rel: "stylesheet",
		href: matches[1],
		getAttribute: getProperty
	  };
	  if (isLocalStylesheet(link)) {
		// Link is local, get the base URL.
		href = getBase(link.href);
		if (href !== false) {
		  stylesheets[href] = link;
		}
	  }
	}
  }
  return stylesheets;
}

var stylesheets = getLocalStylesheets();
/**
 * Reload a stylesheet.
 * FROM Vogue (https://github.com/andrewdavey/vogue)
 * Copyright (c) 2011 Andrew Davey 
 *
 * @param {String} href The URL of the stylesheet to be reloaded.
 */
function reloadStylesheet(href) {
  var newHref = stylesheets[href].href +
		(href.indexOf("?") >= 0 ? "&" : "?") +
		"_vogue_nocache=" + (new Date).getTime(),
	  stylesheet;
  // Check if the appropriate DOM Node is there.
  if (!stylesheets[href].setAttribute) {
	// Create the link.
	stylesheet = document.createElement("link");
	stylesheet.setAttribute("rel", "stylesheet");
	stylesheet.setAttribute("href", newHref);
	head.appendChild(stylesheet);

	// Update the reference to the newly created link.
	stylesheets[href] = stylesheet;
  } else {
	// Update the href to the new URL.
	stylesheets[href].href = newHref;
  }
}


//var http = location.protocol;
//var slashes = http.concat("//");
//var host = slashes.concat(window.location.host);
//var url = host + '/_sockjs';

if (!window.location.origin) { // Some browsers (mainly IE) do not have this property, so we need to build it manually...
  window.location.origin = window.location.protocol + '//' + window.location.hostname + (window.location.port ? (':' + window.location.port) : '');
}
var url = window.location.origin+'/_sockjs'

console.log(url);
var sock = new SockJS(url);
console.log(sock);
 sock.onopen = function() {
     console.log('open');
 };
 sock.onmessage = function(e) {
     console.log('message', e.data);
	 var splits = e.data.split(":");
	 var type = splits[0]
	 var path = splits[1];
	 if(type == "write"){
		if(path.indexOf(".css") == path.length - 4){
			console.log("reloading css : " + path);
			reloadStylesheet(path);
		}
	 }
	 
	 // Reload the current page, without using the cache
	//document.location.reload(false);
 };
 sock.onclose = function() {
     console.log('close');
 };

 //sock.send('test');
 //sock.close();
 
};

window.onload = reload;