const Parser = require('rss-parser');
const fs = require('fs').promises;
const path = require('path');

const FEEDS_FILE = path.join(__dirname, '..', 'feeds.txt');
const DATA_FILE = path.join(__dirname, '..', 'data', 'posts.json');
const MAX_POSTS = 250; // Keep the 250 most recent posts

async function fetchNews() {
    console.log('Starting news fetch process...');

    try {
        // Read feed URLs from feeds.txt
        const feedUrls = (await fs.readFile(FEEDS_FILE, 'utf8'))
            .split('\n')
            .map(url => url.trim())
            .filter(url => url);

        if (feedUrls.length === 0) {
            console.log('No feed URLs found in feeds.txt. Exiting.');
            return;
        }
        
        console.log(`Found ${feedUrls.length} feed(s).`);

        const parser = new Parser({
            timeout: 10000, // Set a timeout for fetching feeds
            headers: {'User-Agent': 'Hugo-Indexer-Pro/1.0'}
        });

        let allPosts = [];

        // Fetch posts from all feeds in parallel
        await Promise.all(
            feedUrls.map(async (feedUrl) => {
                try {
                    const feed = await parser.parseURL(feedUrl);
                    if (feed.items) {
                        console.log(`Fetched ${feed.items.length} items from ${feedUrl}`);
                        const posts = feed.items.map(item => ({
                            title: item.title || 'No title',
                            link: item.link || '',
                            pubDate: item.pubDate ? new Date(item.pubDate).toISOString() : new Date().toISOString(),
                            creator: item.creator || item['dc:creator'] || 'Unknown author',
                            content: item.contentSnippet || item.content || '',
                            source: feed.title || 'Unknown source',
                            sourceUrl: feed.link || feedUrl
                        }));
                        allPosts.push(...posts);
                    }
                } catch (error) {
                    console.error(`Error fetching or parsing feed: ${feedUrl}`, error.message);
                }
            })
        );
        
        console.log(`Fetched a total of ${allPosts.length} posts from all feeds.`);

        // Sort posts by publication date (newest first)
        allPosts.sort((a, b) => new Date(b.pubDate) - new Date(a.pubDate));

        // Limit the number of posts
        const recentPosts = allPosts.slice(0, MAX_POSTS);
        console.log(`Keeping the ${recentPosts.length} most recent posts.`);

        // Write to data/posts.json
        await fs.writeFile(DATA_FILE, JSON.stringify(recentPosts, null, 2));

        console.log(`Successfully updated ${DATA_FILE}`);

    } catch (error) {
        console.error('An error occurred during the fetch process:', error);
        process.exit(1); // Exit with an error code to fail the GitHub Action
    }
}

fetchNews();
