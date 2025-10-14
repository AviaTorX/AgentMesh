class ForceDirectedGraph {
    constructor(svgId) {
        this.svg = d3.select(`#${svgId}`);
        this.width = this.svg.node().clientWidth;
        this.height = this.svg.node().clientHeight;
        
        this.simulation = d3.forceSimulation()
            .force('link', d3.forceLink().id(d => d.id).distance(150))
            .force('charge', d3.forceManyBody().strength(-300))
            .force('center', d3.forceCenter(this.width / 2, this.height / 2))
            .force('collision', d3.forceCollide().radius(40));

        this.linkGroup = this.svg.append('g').attr('class', 'links');
        this.nodeGroup = this.svg.append('g').attr('class', 'nodes');
        this.labelGroup = this.svg.append('g').attr('class', 'labels');

        this.nodes = [];
        this.links = [];
    }

    update(snapshot) {
        if (!snapshot || !snapshot.agents || !snapshot.edges) return;

        // Convert agents to nodes
        const newNodes = Object.values(snapshot.agents).map(agent => ({
            id: agent.id,
            name: agent.name,
            role: agent.role,
            status: agent.status
        }));

        // Convert edges to links
        const newLinks = Object.values(snapshot.edges).map(edge => ({
            source: edge.source_id,
            target: edge.target_id,
            weight: edge.weight,
            usage: edge.usage
        }));

        this.nodes = newNodes;
        this.links = newLinks;

        this.render();
    }

    render() {
        // FORCE COMPLETE CLEANUP - Remove ALL existing elements first
        this.linkGroup.selectAll('line').remove();
        this.nodeGroup.selectAll('circle').remove();
        this.labelGroup.selectAll('text').remove();

        // Rebuild links from scratch
        this.linkGroup
            .selectAll('line')
            .data(this.links, d => `${d.source}-${d.target}`)
            .enter()
            .append('line')
            .attr('class', 'link')
            .attr('stroke-width', d => Math.max(1, d.weight * 5))
            .attr('stroke-opacity', d => 0.3 + (d.weight * 0.7));

        // Rebuild nodes from scratch
        this.nodeGroup
            .selectAll('circle')
            .data(this.nodes, d => d.id)
            .enter()
            .append('circle')
            .attr('r', 20)
            .attr('class', d => `node ${d.role}`)
            .call(this.drag(this.simulation));

        // Rebuild labels from scratch
        this.labelGroup
            .selectAll('text')
            .data(this.nodes, d => d.id)
            .enter()
            .append('text')
            .attr('class', 'node-label')
            .attr('dy', '.35em')
            .text(d => d.name);

        // Update simulation
        this.simulation
            .nodes(this.nodes)
            .on('tick', () => this.ticked());

        this.simulation
            .force('link')
            .links(this.links);

        this.simulation.alpha(0.3).restart();
    }

    ticked() {
        this.linkGroup
            .selectAll('line')
            .attr('x1', d => d.source.x)
            .attr('y1', d => d.source.y)
            .attr('x2', d => d.target.x)
            .attr('y2', d => d.target.y);

        this.nodeGroup
            .selectAll('circle')
            .attr('cx', d => d.x)
            .attr('cy', d => d.y);

        this.labelGroup
            .selectAll('text')
            .attr('x', d => d.x)
            .attr('y', d => d.y + 35);
    }

    drag(simulation) {
        function dragstarted(event) {
            if (!event.active) simulation.alphaTarget(0.3).restart();
            event.subject.fx = event.subject.x;
            event.subject.fy = event.subject.y;
        }

        function dragged(event) {
            event.subject.fx = event.x;
            event.subject.fy = event.y;
        }

        function dragended(event) {
            if (!event.active) simulation.alphaTarget(0);
            event.subject.fx = null;
            event.subject.fy = null;
        }

        return d3.drag()
            .on('start', dragstarted)
            .on('drag', dragged)
            .on('end', dragended);
    }
}
