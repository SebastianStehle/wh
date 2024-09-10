(function () {
    'use strict';

    console.log('Starting live reload check.');

    function checkWebServer() {
        let hasFailed = false;
    
        function connect() {
            const eventSource = new EventSource('/live-reload');
    
            eventSource.onmessage = () => {
                if (hasFailed) {
                    location.reload();
                }
            };
    
            eventSource.onerror = () => {
                eventSource.close();
    
                hasFailed = true;
    
                setTimeout(() => {
                    connect();
                }, [1000]);
            }
        }
    
        connect();
    }

    checkWebServer();
})();