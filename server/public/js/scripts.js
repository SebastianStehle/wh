
(function () {
    htmx.defineExtension('ace', {
        onEvent: function (name, event) {
            const element = event.target;

            if (!element.getAttribute) {
                return;
            }

            const mode = element.getAttribute('hx-mode');

            if (!mode) {
                return;
            }

            switch (name) {
                case 'htmx:afterProcessNode':
                    const editor = ace.edit(element, {
                        minLines: 5,
                        maxLines: 10,
                        fontSize: '11pt',
                        fontFamily: undefined,
                        padding: 8
                    });

                    editor.setTheme('ace/theme/github');
                    editor.session.setMode(mode);
                    editor.renderer.setScrollMargin(10, 10);
                    break;
            }
        }
    });
})();

(function () {
    const openIds = new Set();

    htmx.defineExtension('toggle', {
        onEvent: function (name, event) {
            const element = event.target;
            
            if (!element.getAttribute) {
                return;
            }

            const toggleClass = element.getAttribute('hx-toggle-class');
            const toggleTarget = element.getAttribute('hx-toggle-target');

            if (!toggleClass || !toggleTarget) {
                return;
            }

            const target = element.querySelector(toggleTarget);

            if (!target) {
                return;
            }

            const id = element.id;
            if (!id) {
                return;
            }

            const listener = () => {
                if (openIds.has(id)) {
                    htmx.removeClass(element, toggleClass);
                    openIds.delete(id); 
                } else {
                    htmx.addClass(element, toggleClass);
                    openIds.add(id); 
                }
            };

            switch (name) {
                case 'htmx:beforeCleanupElement':
                    target.removeEventListener('click', listener);
                    break;

                case 'htmx:afterProcessNode':
                    target.addEventListener('click', listener);

                    if (openIds.has(id)) {
                        htmx.addClass(element, openClass);
                    }

                    break;
            }
        }
    });
})();

(function () {
    let api;

    class Loader {
        cancel() {
            this.isCancelled = true;
        }

        async run(element, url) {
            let changeSetId = '';

            while (!this.isCancelled) {
                const response = await fetch(`/events?changeSet=${changeSetId}`);
        
                if (!response.ok) {
                    await delay(1000);
                    continue;
                }
        
                let content = await response.text();

                if (this.isCancelled) {
                    return;
                }
    
                api.withExtensions(element, extension => {
                    content = extension.transformResponse(content, null, element);
                });

                const parser = new DOMParser();
                const parsed = parser.parseFromString(content, 'text/html');
        
                for (const child of parsed.querySelectorAll('.event')) {
                    const id = child.id;
        
                    const existing = document.getElementById(id);
                    if (existing) {
                        existing.parentElement.insertBefore(child, existing);
                        existing.remove();
                    } else {
                        element.prepend(child);
                    }
    
                    htmx.process(child);
                }
        
                changeSetId = response.headers.get('x-changeset');
                await delay(5000);
            }
        }
    }

    htmx.defineExtension('log', {
        init: function (apiRef) {
            api = apiRef;
        },

        onEvent: function (name, event) {
            const element = event.target;
            
            if (!element.getAttribute || !element.getAttribute('hx-events')) {
                return;
            }
        
            switch (name) {
                case 'htmx:beforeCleanupElement':
                    element.loader?.cancel();
                    break;
                case 'htmx:afterProcessNode':
                    element.loader = new Loader();
                    element.loader.run(element);
                    break;
            }
        }
    });
})();

function delay(time) {
    return new Promise(resolve => setInterval(resolve, time));
}